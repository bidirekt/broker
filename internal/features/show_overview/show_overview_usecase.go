package show_overview

import (
	"context"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const panelLimit = 10

type ShowOverviewStatus int

const (
	BrokerFilled ShowOverviewStatus = iota
	BrokerEmpty
)

type ShowOverviewInput struct{}

type ReportedCheck struct {
	Check  model.CompatibilityCheck
	Report breakreport.Report
}

type SaidNoCheck struct {
	ReportedCheck
	DeployedThereAnyway bool
}

type DeployedParticipant struct {
	Participant model.Participant
	Deployments []model.Deployment
}

type Event struct {
	Deployment *model.Deployment
	Check      *ReportedCheck
}

type ShowOverviewOutput struct {
	Status                    ShowOverviewStatus
	Numbers                   model.OverviewNumbers
	SaidNo                    []SaidNoCheck
	SaidNoTotal               int
	Environments              []model.Environment
	Deployed                  []DeployedParticipant
	DeployedParticipantsTotal int
	Activity                  []Event
}

type ShowOverviewUseCase struct {
	overviewRepository *repository.OverviewRepository
}

func NewShowOverviewUseCase(overviewRepository *repository.OverviewRepository) *ShowOverviewUseCase {
	return &ShowOverviewUseCase{overviewRepository: overviewRepository}
}

func (this *ShowOverviewUseCase) Execute(ctx context.Context, _ ShowOverviewInput) ShowOverviewOutput {
	numbers := this.overviewRepository.CountNumbers(ctx)
	if numbers.Participants == 0 || numbers.Environments == 0 {
		return ShowOverviewOutput{Status: BrokerEmpty, Numbers: numbers}
	}

	saidNo, saidNoTotal := this.overviewRepository.ListSaidNoChecks(ctx, panelLimit)
	deployments, deployedParticipantsTotal := this.overviewRepository.ListCurrentDeployments(ctx, panelLimit)
	recentDeployments := this.overviewRepository.ListRecentDeployments(ctx, panelLimit)
	recentChecks := this.overviewRepository.ListRecentChecks(ctx, panelLimit)

	checkIDs := make([]int64, 0, len(saidNo)+len(recentChecks))
	for _, check := range saidNo {
		checkIDs = append(checkIDs, check.ID)
	}
	for _, check := range recentChecks {
		checkIDs = append(checkIDs, check.ID)
	}
	resultsByCheck := this.overviewRepository.ListResultsByCheck(ctx, checkIDs)

	saidNoChecks := make([]SaidNoCheck, 0, len(saidNo))
	for _, check := range saidNo {
		saidNoChecks = append(saidNoChecks, SaidNoCheck{
			ReportedCheck:       toReportedCheck(check.CompatibilityCheck, resultsByCheck[check.ID]),
			DeployedThereAnyway: check.DeployedThereAnyway,
		})
	}

	reportedChecks := make([]ReportedCheck, 0, len(recentChecks))
	for _, check := range recentChecks {
		reportedChecks = append(reportedChecks, toReportedCheck(check, resultsByCheck[check.ID]))
	}

	return ShowOverviewOutput{
		Status:                    BrokerFilled,
		Numbers:                   numbers,
		SaidNo:                    saidNoChecks,
		SaidNoTotal:               saidNoTotal,
		Environments:              this.overviewRepository.ListEnvironments(ctx),
		Deployed:                  toDeployedParticipants(deployments),
		DeployedParticipantsTotal: deployedParticipantsTotal,
		Activity:                  toNewestEvents(recentDeployments, reportedChecks),
	}
}

func toDeployedParticipants(deployments []model.Deployment) []DeployedParticipant {
	participants := make([]DeployedParticipant, 0)
	for _, deployment := range deployments {
		last := len(participants) - 1
		if last < 0 || participants[last].Participant.ID != deployment.Participant.ID {
			participants = append(participants, DeployedParticipant{Participant: *deployment.Participant})
			last++
		}
		participants[last].Deployments = append(participants[last].Deployments, deployment)
	}

	return participants
}

func toNewestEvents(deployments []model.Deployment, checks []ReportedCheck) []Event {
	events := make([]Event, 0, panelLimit)
	for len(events) < panelLimit && (len(deployments) > 0 || len(checks) > 0) {
		if len(checks) == 0 || (len(deployments) > 0 && !deployments[0].DeployedAt.Before(checks[0].Check.CreatedAt)) {
			events = append(events, Event{Deployment: &deployments[0]})
			deployments = deployments[1:]
		} else {
			events = append(events, Event{Check: &checks[0]})
			checks = checks[1:]
		}
	}

	return events
}

func toReportedCheck(check model.CompatibilityCheck, results []model.CompatibilityCheckResult) ReportedCheck {
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

	return ReportedCheck{Check: check, Report: report}
}
