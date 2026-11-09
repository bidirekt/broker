package list_participant_checks

import (
	"context"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ListParticipantChecksStatus int

const (
	ParticipantChecksListed ListParticipantChecksStatus = iota
	ParticipantNotFound
)

type ListParticipantChecksInput struct {
	Name string
	Page int64
}

type ListParticipantChecksOutput struct {
	Status ListParticipantChecksStatus
	Header *repository.ParticipantChecksHeader
	Checks []ListedCheck
	Total  int
}

type ListedCheck struct {
	Check  model.CompatibilityCheck
	Report breakreport.Report
}

type ListParticipantChecksUseCase struct {
	participantChecksRepository *repository.ParticipantChecksRepository
	reportReader                *breakreport.Reader
}

func NewListParticipantChecksUseCase(
	participantChecksRepository *repository.ParticipantChecksRepository,
	reportReader *breakreport.Reader,
) *ListParticipantChecksUseCase {
	return &ListParticipantChecksUseCase{participantChecksRepository: participantChecksRepository, reportReader: reportReader}
}

func (this *ListParticipantChecksUseCase) Execute(ctx context.Context, input ListParticipantChecksInput) ListParticipantChecksOutput {
	header, found := this.participantChecksRepository.FindHeader(ctx, input.Name)
	if !found {
		return ListParticipantChecksOutput{Status: ParticipantNotFound}
	}

	checks, total := this.participantChecksRepository.ListChecks(ctx, header.Participant.ID, pages.PageSize, pages.Offset(input.Page))
	checkIDs := make([]int64, 0, len(checks))
	for _, check := range checks {
		checkIDs = append(checkIDs, check.ID)
	}
	resultsByCheck := this.participantChecksRepository.ListResultsByCheck(ctx, checkIDs)

	listed := make([]ListedCheck, 0, len(checks))
	for _, check := range checks {
		listed = append(listed, ListedCheck{Check: check, Report: this.reportReader.FromStoredCheck(check, resultsByCheck[check.ID])})
	}

	return ListParticipantChecksOutput{Status: ParticipantChecksListed, Header: header, Checks: listed, Total: total}
}
