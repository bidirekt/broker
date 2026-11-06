package list_participant_pages

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ListParticipantPagesStatus int

const (
	ParticipantsListed ListParticipantPagesStatus = iota
)

type ListParticipantPagesInput struct {
	Page int64
}

type ListParticipantPagesOutput struct {
	Status       ListParticipantPagesStatus
	Participants []*model.ParticipantListing
	Total        int
}

type ListParticipantPagesUseCase struct {
	participantListingRepository *repository.ParticipantListingRepository
}

func NewListParticipantPagesUseCase(participantListingRepository *repository.ParticipantListingRepository) *ListParticipantPagesUseCase {
	return &ListParticipantPagesUseCase{participantListingRepository: participantListingRepository}
}

func (this *ListParticipantPagesUseCase) Execute(ctx context.Context, input ListParticipantPagesInput) ListParticipantPagesOutput {
	participants, total := this.participantListingRepository.List(ctx, pages.PageSize, pages.Offset(input.Page))

	return ListParticipantPagesOutput{Status: ParticipantsListed, Participants: participants, Total: total}
}
