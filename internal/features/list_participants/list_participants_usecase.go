package list_participants

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
)

type ListParticipantsStatus int

const (
	ParticipantsListed ListParticipantsStatus = iota
)

type ListParticipantsInput struct{}

type ListParticipantsOutput struct {
	Status       ListParticipantsStatus
	Participants []string
}

type ListParticipantsUseCase struct {
	participantRepository *repository.ParticipantRepository
}

func NewListParticipantsUseCase(participantRepository *repository.ParticipantRepository) *ListParticipantsUseCase {
	return &ListParticipantsUseCase{participantRepository: participantRepository}
}

func (this *ListParticipantsUseCase) Execute(ctx context.Context, input ListParticipantsInput) ListParticipantsOutput {
	return ListParticipantsOutput{
		Status:       ParticipantsListed,
		Participants: this.participantRepository.ListNames(ctx),
	}
}
