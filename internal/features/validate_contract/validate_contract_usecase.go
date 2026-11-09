package validate_contract

import (
	"context"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ValidateContractStatus int

const (
	ContractValidated ValidateContractStatus = iota
	ParticipantNotFound
	EnvironmentNotFound
	ContractFilesUnreadable
	ContractValidationFailed
)

type ValidateContractInput struct {
	ParticipantName string
	EnvironmentName string
	Contracts       []model.ContractFile
}

type ValidateContractOutput struct {
	Status      ValidateContractStatus
	ReadError   string
	Violations  []violation.Violation
	Participant string
	Environment string
	Deployable  bool
	Results     map[string]compatibility_checker.Hierarchical
}

type ValidateContractUseCase struct {
	contractRepository    *repository.ContractRepository
	deploymentRepository  *repository.DeploymentRepository
	environmentRepository *repository.EnvironmentRepository
	participantRepository *repository.ParticipantRepository
	compatibilityChecker  *compatibility_checker.CompatibilityChecker
}

func NewValidateContractUseCase(
	contractRepository *repository.ContractRepository,
	deploymentRepository *repository.DeploymentRepository,
	environmentRepository *repository.EnvironmentRepository,
	participantRepository *repository.ParticipantRepository,
	compatibilityChecker *compatibility_checker.CompatibilityChecker,
) *ValidateContractUseCase {
	return &ValidateContractUseCase{
		contractRepository:    contractRepository,
		deploymentRepository:  deploymentRepository,
		environmentRepository: environmentRepository,
		participantRepository: participantRepository,
		compatibilityChecker:  compatibilityChecker,
	}
}

func (this *ValidateContractUseCase) Execute(ctx context.Context, input ValidateContractInput) (ValidateContractOutput, error) {
	participant, exists := this.participantRepository.FindByName(ctx, input.ParticipantName)
	if !exists {
		return ValidateContractOutput{Status: ParticipantNotFound}, nil
	}

	environment, exists := this.environmentRepository.FindByName(ctx, input.EnvironmentName)
	if !exists {
		return ValidateContractOutput{Status: EnvironmentNotFound}, nil
	}

	fragments, shapeViolations, err := contractfiles.ToFragments(input.Contracts)
	if err != nil {
		return ValidateContractOutput{Status: ContractFilesUnreadable, ReadError: err.Error()}, nil
	}

	if len(shapeViolations) > 0 {
		return ValidateContractOutput{Status: ContractValidationFailed, Violations: shapeViolations}, nil
	}

	uploadedContract, violations, err := contractfiles.ToUploadedContract(fragments, participant, "", nil)
	if len(violations) > 0 {
		return ValidateContractOutput{Status: ContractValidationFailed, Violations: violations}, nil
	}

	if err != nil {
		return ValidateContractOutput{}, err
	}

	removedResources := this.toRemovedSinceDeployed(ctx, participant, environment, uploadedContract)

	localContract := toPersistedContract(uploadedContract, removedResources)

	counterparts := this.contractRepository.LoadCounterparts(ctx, localContract, environment.ID)
	compatibilityReport := this.compatibilityChecker.Check(ctx, localContract, environment, counterparts)

	return ValidateContractOutput{
		Status:      ContractValidated,
		Participant: participant.Name,
		Environment: environment.Name,
		Deployable:  compatibilityReport.Deployable(),
		Results:     compatibilityReport.Hierarchical,
	}, nil
}

func (this *ValidateContractUseCase) toRemovedSinceDeployed(
	ctx context.Context,
	participant *model.Participant,
	environment *model.Environment,
	uploadedContract *model.UploadedContract,
) map[string]model.PersistedResource {
	version, contractID, deployed := this.deploymentRepository.CurrentDeploymentInEnv(ctx, participant.ID, environment.ID)
	if !deployed {
		return nil
	}

	deployedContract := this.contractRepository.GetContractByIDAndVersion(ctx, contractID, version)

	return contract_differ.RemovedSinceDeployed(deployedContract, uploadedContract)
}

func toPersistedContract(
	uploadedContract *model.UploadedContract,
	removedResources map[string]model.PersistedResource,
) *model.PersistedContract {
	resources := make(map[string]model.PersistedResource, len(uploadedContract.Resources)+len(removedResources))
	for hash, resource := range uploadedContract.Resources {
		resources[hash] = model.PersistedResource{
			Resource:     resource.Resource,
			Schemas:      uploadedContract.Schemas,
			ProviderHash: resource.ProviderHash(),
		}
	}

	for hash, resource := range removedResources {
		resources[hash] = resource
	}

	return &model.PersistedContract{
		ParticipantName: uploadedContract.ParticipantName,
		Resources:       resources,
		Schemas:         uploadedContract.Schemas,
	}
}
