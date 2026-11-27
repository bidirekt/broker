package create_participant

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type CreateParticipantStatus int

const (
	ParticipantCreated CreateParticipantStatus = iota
	ParticipantNameInvalid
	ParticipantAlreadyExists
	ParticipantNameReserved
)

type CreateParticipantInput struct {
	Participant string
}

// Message is the reserving protocol's reason when the Status is ParticipantNameReserved.
type CreateParticipantOutput struct {
	Status  CreateParticipantStatus
	Message string
}

type CreateParticipantUseCase struct {
	participantRepository *repository.ParticipantRepository
	reservedNames         []repository.ReservedNames
}

func NewCreateParticipantUseCase(participantRepository *repository.ParticipantRepository, reservedNames []repository.ReservedNames) *CreateParticipantUseCase {
	return &CreateParticipantUseCase{participantRepository: participantRepository, reservedNames: reservedNames}
}

func (this *CreateParticipantUseCase) Execute(ctx context.Context, input CreateParticipantInput) CreateParticipantOutput {
	if validations.ParticipantName(input.Participant) != nil {
		return CreateParticipantOutput{Status: ParticipantNameInvalid}
	}

	result := this.participantRepository.Create(ctx, model.NewParticipant(input.Participant), this.reservedNames)

	switch result.Outcome {
	case repository.ParticipantExists:
		return CreateParticipantOutput{Status: ParticipantAlreadyExists}
	case repository.ParticipantNameReserved:
		return CreateParticipantOutput{Status: ParticipantNameReserved, Message: result.Message}
	default:
		return CreateParticipantOutput{Status: ParticipantCreated}
	}
}
