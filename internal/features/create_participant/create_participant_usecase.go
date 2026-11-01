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
)

type CreateParticipantInput struct {
	Participant string
}

type CreateParticipantOutput struct {
	Status CreateParticipantStatus
}

type CreateParticipantUseCase struct {
	participantRepository *repository.ParticipantRepository
}

func NewCreateParticipantUseCase(participantRepository *repository.ParticipantRepository) *CreateParticipantUseCase {
	return &CreateParticipantUseCase{participantRepository: participantRepository}
}

func (this *CreateParticipantUseCase) Execute(ctx context.Context, input CreateParticipantInput) CreateParticipantOutput {
	if validations.ParticipantName(input.Participant) != nil {
		return CreateParticipantOutput{Status: ParticipantNameInvalid}
	}

	created := this.participantRepository.Create(ctx, model.NewParticipant(input.Participant))
	if !created {
		return CreateParticipantOutput{Status: ParticipantAlreadyExists}
	}

	return CreateParticipantOutput{Status: ParticipantCreated}
}
