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
	ContractSyntaxValidated
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
	interpreter           *contractfiles.Interpreter
	contractRepository    *repository.ContractRepository
	deploymentRepository  *repository.DeploymentRepository
	environmentRepository *repository.EnvironmentRepository
	participantRepository *repository.ParticipantRepository
	compatibilityChecker  *compatibility_checker.CompatibilityChecker
}

func NewValidateContractUseCase(
	interpreter *contractfiles.Interpreter,
	contractRepository *repository.ContractRepository,
	deploymentRepository *repository.DeploymentRepository,
	environmentRepository *repository.EnvironmentRepository,
	participantRepository *repository.ParticipantRepository,
	compatibilityChecker *compatibility_checker.CompatibilityChecker,
) *ValidateContractUseCase {
	return &ValidateContractUseCase{
		interpreter:           interpreter,
		contractRepository:    contractRepository,
		deploymentRepository:  deploymentRepository,
		environmentRepository: environmentRepository,
		participantRepository: participantRepository,
		compatibilityChecker:  compatibilityChecker,
	}
}

func (this *ValidateContractUseCase) Execute(ctx context.Context, input ValidateContractInput) (ValidateContractOutput, error) {
	if input.EnvironmentName == "" {
		return this.validateSyntax(ctx, input.Contracts), nil
	}

	participant, exists := this.participantRepository.FindByName(ctx, input.ParticipantName)
	if !exists {
		return ValidateContractOutput{Status: ParticipantNotFound}, nil
	}

	environment, exists := this.environmentRepository.FindByName(ctx, input.EnvironmentName)
	if !exists {
		return ValidateContractOutput{Status: EnvironmentNotFound}, nil
	}

	uploadedContract, violations, readErr, err := this.interpreter.ToUploadedContract(input.Contracts, participant, "")
	if readErr != nil {
		return ValidateContractOutput{Status: ContractFilesUnreadable, ReadError: readErr.Error()}, nil
	}

	if len(violations) > 0 {
		return ValidateContractOutput{Status: ContractValidationFailed, Violations: violations}, nil
	}

	if err != nil {
		return ValidateContractOutput{}, err
	}

	removedResources := this.toRemovedSinceDeployed(ctx, participant, environment, uploadedContract)

	localContract := toPersistedContract(uploadedContract, removedResources)

	if refusal, refused := this.contractRepository.RefuseContractExtensions(ctx, localContract); refused {
		return ValidateContractOutput{Status: ContractValidationFailed, Violations: []violation.Violation{violation.FromRefusal(refusal)}}, nil
	}

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

func (this *ValidateContractUseCase) validateSyntax(ctx context.Context, files []model.ContractFile) ValidateContractOutput {
	extensions, violations, readErr := this.interpreter.ToViolations(files)
	if readErr != nil {
		return ValidateContractOutput{Status: ContractFilesUnreadable, ReadError: readErr.Error()}
	}

	if len(violations) > 0 {
		return ValidateContractOutput{Status: ContractValidationFailed, Violations: violations}
	}

	if refusal, refused := this.contractRepository.RefuseContractExtensions(ctx, &model.PersistedContract{Extensions: extensions}); refused {
		return ValidateContractOutput{Status: ContractValidationFailed, Violations: []violation.Violation{violation.FromRefusal(refusal)}}
	}

	return ValidateContractOutput{Status: ContractSyntaxValidated}
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
			Schemas:      model.CatalogFor(uploadedContract.Schemas, resource.Bodies),
			ProviderHash: resource.ProviderHash(),
		}
	}

	for hash, resource := range removedResources {
		resources[hash] = resource
	}

	return &model.PersistedContract{
		ParticipantID:   uploadedContract.ParticipantID,
		ParticipantName: uploadedContract.ParticipantName,
		Resources:       resources,
		Schemas:         uploadedContract.Schemas,
		Extensions:      uploadedContract.Extensions,
	}
}
