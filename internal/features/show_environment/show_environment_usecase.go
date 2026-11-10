package show_environment

import (
	"context"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ShowEnvironmentStatus int

const (
	EnvironmentFound ShowEnvironmentStatus = iota
	EnvironmentNotFound
)

type ShowEnvironmentInput struct {
	EnvironmentID int64
	HistoryPage   int64
	ChecksPage    int64
}

type ReportedCheck struct {
	Check  model.CompatibilityCheck
	Report breakreport.Report
}

type ShowEnvironmentOutput struct {
	Status           ShowEnvironmentStatus
	Environment      *model.Environment
	Deployed         []model.Deployment
	Deployments      []model.Deployment
	DeploymentsTotal int
	Checks           []ReportedCheck
	ChecksTotal      int
}

type ShowEnvironmentUseCase struct {
	environmentPageRepository *repository.EnvironmentPageRepository
	deploymentRepository      *repository.DeploymentRepository
	reportReader              *breakreport.Reader
}

func NewShowEnvironmentUseCase(
	environmentPageRepository *repository.EnvironmentPageRepository,
	deploymentRepository *repository.DeploymentRepository,
	reportReader *breakreport.Reader,
) *ShowEnvironmentUseCase {
	return &ShowEnvironmentUseCase{
		environmentPageRepository: environmentPageRepository,
		deploymentRepository:      deploymentRepository,
		reportReader:              reportReader,
	}
}

func (this *ShowEnvironmentUseCase) Execute(ctx context.Context, input ShowEnvironmentInput) ShowEnvironmentOutput {
	environment, found := this.environmentPageRepository.FindEnvironment(ctx, input.EnvironmentID)
	if !found {
		return ShowEnvironmentOutput{Status: EnvironmentNotFound}
	}

	deployments, deploymentsTotal := this.environmentPageRepository.ListDeployments(
		ctx, environment.ID, pages.PageSize, pages.Offset(input.HistoryPage),
	)
	checks, checksTotal := this.environmentPageRepository.ListChecks(
		ctx, environment.ID, pages.PageSize, pages.Offset(input.ChecksPage),
	)

	return ShowEnvironmentOutput{
		Status:           EnvironmentFound,
		Environment:      environment,
		Deployed:         this.deploymentRepository.ListCurrentDeploymentsInEnv(ctx, environment.ID),
		Deployments:      deployments,
		DeploymentsTotal: deploymentsTotal,
		Checks:           this.toReportedChecks(ctx, checks),
		ChecksTotal:      checksTotal,
	}
}

func (this *ShowEnvironmentUseCase) toReportedChecks(ctx context.Context, checks []model.CompatibilityCheck) []ReportedCheck {
	checkIDs := make([]int64, 0, len(checks))
	for _, check := range checks {
		checkIDs = append(checkIDs, check.ID)
	}
	resultsByCheck := this.environmentPageRepository.ListResultsByCheck(ctx, checkIDs)

	reported := make([]ReportedCheck, 0, len(checks))
	for _, check := range checks {
		reported = append(reported, ReportedCheck{Check: check, Report: this.reportReader.FromStoredCheck(check, resultsByCheck[check.ID])})
	}

	return reported
}
