package show_participant_versions

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ShowParticipantVersionsStatus int

const (
	ParticipantVersionsFound ShowParticipantVersionsStatus = iota
	ParticipantVersionsParticipantNotFound
)

type ShowParticipantVersionsInput struct {
	Name string
	Page int64
}

type ShowParticipantVersionsOutput struct {
	Status   ShowParticipantVersionsStatus
	Header   *model.ParticipantVersionsHeader
	Versions []model.PublishedVersion
	Total    int
}

type ShowParticipantVersionsUseCase struct {
	participantVersionsRepository *repository.ParticipantVersionsRepository
}

func NewShowParticipantVersionsUseCase(participantVersionsRepository *repository.ParticipantVersionsRepository) *ShowParticipantVersionsUseCase {
	return &ShowParticipantVersionsUseCase{participantVersionsRepository: participantVersionsRepository}
}

func (this *ShowParticipantVersionsUseCase) Execute(ctx context.Context, input ShowParticipantVersionsInput) ShowParticipantVersionsOutput {
	header, found := this.participantVersionsRepository.FindHeader(ctx, input.Name)
	if !found {
		return ShowParticipantVersionsOutput{Status: ParticipantVersionsParticipantNotFound}
	}

	versions, total := this.participantVersionsRepository.ListPublishedVersions(ctx, header.Participant.ID, pages.PageSize, pages.Offset(input.Page))

	return ShowParticipantVersionsOutput{Status: ParticipantVersionsFound, Header: header, Versions: versions, Total: total}
}
