package validate_contract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

type ValidateContractHandler struct {
	contractRepository    *repository.ContractRepository
	deploymentRepository  *repository.DeploymentRepository
	environmentRepository *repository.EnvironmentRepository
	participantRepository *repository.ParticipantRepository
	compatibilityChecker  *compatibility_checker.CompatibilityChecker
}

func NewValidateContractHandler(
	contractRepository *repository.ContractRepository,
	deploymentRepository *repository.DeploymentRepository,
	environmentRepository *repository.EnvironmentRepository,
	participantRepository *repository.ParticipantRepository,
) *ValidateContractHandler {
	return &ValidateContractHandler{
		contractRepository:    contractRepository,
		deploymentRepository:  deploymentRepository,
		environmentRepository: environmentRepository,
		participantRepository: participantRepository,
		compatibilityChecker:  compatibility_checker.NewCompatibilityChecker(verdictsNeverFound{}),
	}
}

func (this *ValidateContractHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &ValidateContractRequestBody{}
	if err := json.Unmarshal(ctx.Body(), requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	participantName := strings.TrimSpace(requestBody.Participant)
	environmentName := strings.TrimSpace(requestBody.Environment)
	if participantName == "" || environmentName == "" || len(requestBody.Contracts) == 0 {
		return this.respondInvalidInput(ctx)
	}

	fragments, shapeViolations, err := contractfiles.ToFragments(requestBody.Contracts)
	if errors.Is(err, contractfiles.ErrBlankSource) {
		return this.respondInvalidInput(ctx)
	}

	if err != nil {
		return this.respondBadRequest(ctx, err)
	}

	if len(shapeViolations) > 0 {
		return this.respondValidationFailed(ctx, shapeViolations)
	}

	participant, exists := this.participantRepository.FindByName(ctx.Context(), participantName)
	if !exists {
		return this.respondNotFound(ctx, ParticipantNotFound)
	}

	environment, exists := this.environmentRepository.FindByName(ctx.Context(), environmentName)
	if !exists {
		return this.respondNotFound(ctx, EnvironmentNotFound)
	}

	uploadedContract, violations, err := contractfiles.ToUploadedContract(fragments, participant, "", "")
	if len(violations) > 0 {
		return this.respondValidationFailed(ctx, violations)
	}

	if err != nil {
		return err
	}

	removedResources, err := this.toRemovedSinceDeployed(ctx.Context(), participant, environment, uploadedContract)
	if err != nil {
		return err
	}

	localContract := toPersistedContract(uploadedContract, removedResources)

	counterparts := this.contractRepository.LoadCounterparts(ctx.Context(), localContract, environment.ID)
	compatibilityReport := this.compatibilityChecker.Check(ctx.Context(), localContract, environment, counterparts)

	deployable := true
	for _, result := range compatibilityReport.Results {
		deployable = deployable && result.Deployable
	}

	return ctx.Status(fiber.StatusOK).JSON(ValidateContractResponseBody{
		Message:     ContractValidated,
		Participant: participant.Name,
		Environment: environment.Name,
		Deployable:  deployable,
		Results:     compatibilityReport.Hierarchical,
	})
}

func (this *ValidateContractHandler) toRemovedSinceDeployed(
	ctx context.Context,
	participant *model.Participant,
	environment *model.Environment,
	uploadedContract *model.UploadedContract,
) (map[string]model.PersistedResource, error) {
	removedResources := make(map[string]model.PersistedResource)

	version, deployed := this.deploymentRepository.CurrentVersionInEnv(ctx, participant.ID, environment.ID)
	if !deployed {
		return removedResources, nil
	}

	deployedContract, exists := this.contractRepository.GetContractByNameAndVersion(ctx, participant.Name, version)
	if !exists {
		return nil, fmt.Errorf("deployed version %q of participant %q has no contract", version, participant.Name)
	}

	deployedProperties := make(map[string]model.ResourceProperties, len(deployedContract.Resources))
	for hash, resource := range deployedContract.Resources {
		if !resource.Removed {
			deployedProperties[hash] = resource.Properties
		}
	}

	localProperties := make(map[string]model.ResourceProperties, len(uploadedContract.Resources))
	for hash, resource := range uploadedContract.Resources {
		localProperties[hash] = resource.Properties
	}

	for hash, change := range contract_differ.DiffResourceProperties(deployedProperties, localProperties).Resources {
		if change.Kind != model.ChangeRemoved {
			continue
		}

		removedResource := deployedContract.Resources[hash]
		removedResource.Removed = true
		removedResources[hash] = removedResource
	}

	return removedResources, nil
}

func toPersistedContract(
	uploadedContract *model.UploadedContract,
	removedResources map[string]model.PersistedResource,
) *model.PersistedContract {
	resources := make(map[string]model.PersistedResource, len(uploadedContract.Resources)+len(removedResources))
	for hash, resource := range uploadedContract.Resources {
		resources[hash] = model.PersistedResource{
			ParticipantName:    resource.ParticipantName,
			Direction:          resource.Direction,
			Interaction:        resource.Interaction,
			ConsumedProvider:   resource.ConsumedProvider,
			Endpoint:           resource.Endpoint,
			Method:             resource.Method,
			ResponseStatusCode: resource.ResponseStatusCode,
			Properties:         resource.Properties,
			ProviderHash:       resource.ProviderHash(),
		}
	}

	for hash, resource := range removedResources {
		resources[hash] = resource
	}

	return &model.PersistedContract{
		ParticipantName: uploadedContract.ParticipantName,
		Resources:       resources,
	}
}

// verdictsNeverFound keeps the pair cache out: local files are no stored snapshot, so no
// verdict can describe them.
type verdictsNeverFound struct{}

func (this verdictsNeverFound) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

func (this *ValidateContractHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(ValidateContractErrorResponseBody{
		Message: ContractInvalidInput,
	})
}

func (this *ValidateContractHandler) respondBadRequest(ctx fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(ValidateContractErrorResponseBody{
		Message: err.Error(),
	})
}

func (this *ValidateContractHandler) respondValidationFailed(ctx fiber.Ctx, violations []violation.Violation) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(ValidateContractValidationResponseBody{
		Message:    ContractValidationFailed,
		Violations: violations,
	})
}

func (this *ValidateContractHandler) respondNotFound(ctx fiber.Ctx, message string) error {
	return ctx.Status(fiber.StatusNotFound).JSON(ValidateContractErrorResponseBody{
		Message: message,
	})
}
