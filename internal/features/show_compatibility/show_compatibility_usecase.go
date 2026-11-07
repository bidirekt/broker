package show_compatibility

import (
	"context"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ShowCompatibilityStatus int

const (
	CompatibilityShown ShowCompatibilityStatus = iota
	NoParticipants
	ParticipantNotFound
)

const allVersionsPageSize = 20

type ShowCompatibilityInput struct {
	Participant      string
	ParticipantGiven bool
	Versions         VersionsFilter
}

type VersionsFilter struct {
	All    bool
	Latest int
	Page   int64
}

type ReportedCheck struct {
	Check  model.CompatibilityCheck
	Report breakreport.Report
}

type ShowCompatibilityOutput struct {
	Status        ShowCompatibilityStatus
	Participants  []string
	Participant   *model.Participant
	Environments  []model.Environment
	Versions      []model.PublishedVersion
	VersionsTotal int
	Checks        []ReportedCheck
}

type ShowCompatibilityUseCase struct {
	participantRepository       *repository.ParticipantRepository
	compatibilityPageRepository *repository.CompatibilityPageRepository
}

func NewShowCompatibilityUseCase(
	participantRepository *repository.ParticipantRepository,
	compatibilityPageRepository *repository.CompatibilityPageRepository,
) *ShowCompatibilityUseCase {
	return &ShowCompatibilityUseCase{
		participantRepository:       participantRepository,
		compatibilityPageRepository: compatibilityPageRepository,
	}
}

func (this *ShowCompatibilityUseCase) Execute(ctx context.Context, input ShowCompatibilityInput) ShowCompatibilityOutput {
	participants := this.participantRepository.ListNames(ctx)
	if len(participants) == 0 && !input.ParticipantGiven {
		return ShowCompatibilityOutput{Status: NoParticipants, Participants: participants}
	}

	name := input.Participant
	if !input.ParticipantGiven {
		name = participants[0]
	}

	participant, found := this.compatibilityPageRepository.FindParticipant(ctx, name)
	if !found {
		return ShowCompatibilityOutput{Status: ParticipantNotFound}
	}

	versions, versionsTotal := this.listVersions(ctx, participant.ID, input.Versions)
	environments := this.compatibilityPageRepository.ListEnvironments(ctx)

	output := ShowCompatibilityOutput{
		Status:        CompatibilityShown,
		Participants:  participants,
		Participant:   participant,
		Environments:  environments,
		Versions:      versions,
		VersionsTotal: versionsTotal,
	}
	if len(versions) > 0 && len(environments) > 0 {
		output.Checks = this.listReportedChecks(ctx, participant.ID, versions)
	}

	return output
}

func (this *ShowCompatibilityUseCase) listVersions(ctx context.Context, participantID int64, filter VersionsFilter) ([]model.PublishedVersion, int) {
	if filter.All {
		return this.compatibilityPageRepository.ListVersions(
			ctx, participantID, allVersionsPageSize, pages.OffsetOfSize(filter.Page, allVersionsPageSize),
		)
	}

	versions := this.compatibilityPageRepository.ListLatestVersions(ctx, participantID, filter.Latest)

	return versions, len(versions)
}

func (this *ShowCompatibilityUseCase) listReportedChecks(ctx context.Context, participantID int64, versions []model.PublishedVersion) []ReportedCheck {
	versionNames := make([]string, 0, len(versions))
	for _, version := range versions {
		versionNames = append(versionNames, version.Version)
	}

	checks := this.compatibilityPageRepository.ListLatestChecks(ctx, participantID, versionNames)
	checkIDs := make([]int64, 0, len(checks))
	for _, check := range checks {
		checkIDs = append(checkIDs, check.ID)
	}
	resultsByCheck := this.compatibilityPageRepository.ListResultsByCheck(ctx, checkIDs)

	reported := make([]ReportedCheck, 0, len(checks))
	for _, check := range checks {
		reported = append(reported, ReportedCheck{
			Check:  check,
			Report: toReport(check, resultsByCheck[check.ID]),
		})
	}

	return reported
}

func toReport(check model.CompatibilityCheck, results []model.CompatibilityCheckResult) breakreport.Report {
	counterparts := make([]breakreport.StoredCounterpart, 0, len(results))
	for _, result := range results {
		counterparts = append(counterparts, breakreport.StoredCounterpart{
			Name:       result.CounterpartName,
			Role:       result.Role,
			Version:    result.CounterpartVersion.String,
			Registered: result.CounterpartParticipantID.Valid,
			Deployable: result.Deployable,
		})
	}

	report, err := breakreport.FromStoredCheck(breakreport.StoredCheck{
		Participant:  check.ParticipantName,
		Environment:  check.EnvironmentName,
		Report:       check.Report,
		Counterparts: counterparts,
	})
	if err != nil {
		panic(err)
	}

	return report
}
