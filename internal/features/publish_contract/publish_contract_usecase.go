package publish_contract

import (
	"context"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type PublishContractStatus int

const (
	ContractPublishSuccessful PublishContractStatus = iota
	ContractParticipantNotFound
	ContractFilesUnreadable
	ContractValidationFailed
	ContractVersionConflict
)

type PublishContractInput struct {
	ServiceName string
	Version     string
	Contracts   []model.ContractFile
}

type PublishContractOutput struct {
	Status     PublishContractStatus
	ReadError  string
	Violations []violation.Violation
}

type PublishContractUseCase struct {
	interpreter           *contractfiles.Interpreter
	contractRepository    *repository.ContractRepository
	participantRepository *repository.ParticipantRepository
}

func NewPublishContractUseCase(
	interpreter *contractfiles.Interpreter,
	contractRepository *repository.ContractRepository,
	participantRepository *repository.ParticipantRepository,
) *PublishContractUseCase {
	return &PublishContractUseCase{
		interpreter:           interpreter,
		contractRepository:    contractRepository,
		participantRepository: participantRepository,
	}
}

func (this *PublishContractUseCase) Execute(ctx context.Context, input PublishContractInput) (PublishContractOutput, error) {
	participant, exists := this.participantRepository.FindByName(ctx, input.ServiceName)
	if !exists {
		return PublishContractOutput{Status: ContractParticipantNotFound}, nil
	}

	uploadedContract, violations, readErr, err := this.interpreter.ToUploadedContract(input.Contracts, participant, input.Version)
	if readErr != nil {
		return PublishContractOutput{Status: ContractFilesUnreadable, ReadError: readErr.Error()}, nil
	}

	if len(violations) > 0 {
		return PublishContractOutput{Status: ContractValidationFailed, Violations: violations}, nil
	}

	if err != nil {
		return PublishContractOutput{}, err
	}

	conflict := this.contractRepository.Publish(ctx, uploadedContract,
		func(current *model.PersistedContract) model.ContractDiff {
			return contract_differ.DiffContracts(current, uploadedContract)
		},
		func(stored []model.ContractFile) bool {
			return this.hasSameContent(stored, participant, uploadedContract)
		},
	)
	if conflict {
		return PublishContractOutput{Status: ContractVersionConflict}, nil
	}

	return PublishContractOutput{Status: ContractPublishSuccessful}, nil
}

func (this *PublishContractUseCase) hasSameContent(stored []model.ContractFile, participant *model.Participant, uploadedContract *model.UploadedContract) bool {
	if slices.Equal(stored, uploadedContract.Files) {
		return true
	}

	storedContract, violations, readErr, err := this.interpreter.ToUploadedContract(stored, participant, uploadedContract.Version)

	return readErr == nil && err == nil && len(violations) == 0 && storedContract.Checksum() == uploadedContract.Checksum()
}
