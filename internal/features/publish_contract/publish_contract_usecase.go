package publish_contract

import (
	"context"
	"encoding/json"

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
	Contracts   []contractfiles.File
}

type PublishContractOutput struct {
	Status     PublishContractStatus
	ReadError  string
	Violations []violation.Violation
}

type PublishContractUseCase struct {
	contractRepository    *repository.ContractRepository
	participantRepository *repository.ParticipantRepository
}

func NewPublishContractUseCase(
	contractRepository *repository.ContractRepository,
	participantRepository *repository.ParticipantRepository,
) *PublishContractUseCase {
	return &PublishContractUseCase{
		contractRepository:    contractRepository,
		participantRepository: participantRepository,
	}
}

func (this *PublishContractUseCase) Execute(ctx context.Context, input PublishContractInput) (PublishContractOutput, error) {
	participant, exists := this.participantRepository.FindByName(ctx, input.ServiceName)
	if !exists {
		return PublishContractOutput{Status: ContractParticipantNotFound}, nil
	}

	fragments, shapeViolations, err := contractfiles.ToFragments(input.Contracts)
	if err != nil {
		return PublishContractOutput{Status: ContractFilesUnreadable, ReadError: err.Error()}, nil
	}

	if len(shapeViolations) > 0 {
		return PublishContractOutput{Status: ContractValidationFailed, Violations: shapeViolations}, nil
	}

	contractContent, _ := json.Marshal(input.Contracts)

	uploadedContract, violations, err := contractfiles.ToUploadedContract(fragments, participant, input.Version, string(contractContent))
	if len(violations) > 0 {
		return PublishContractOutput{Status: ContractValidationFailed, Violations: violations}, nil
	}

	if err != nil {
		return PublishContractOutput{}, err
	}

	found, conflict := this.contractRepository.Publish(ctx, uploadedContract, func(current *model.PersistedContract) model.ContractDiff {
		return contract_differ.DiffContracts(current, uploadedContract)
	})
	if !found {
		return PublishContractOutput{Status: ContractParticipantNotFound}, nil
	}

	if conflict {
		return PublishContractOutput{Status: ContractVersionConflict}, nil
	}

	return PublishContractOutput{Status: ContractPublishSuccessful}, nil
}
