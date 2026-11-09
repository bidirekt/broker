package show_contract_version

import (
	"context"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ShowContractVersionStatus int

const (
	ContractVersionFound ShowContractVersionStatus = iota
	ContractVersionNotFound
)

type ShowContractVersionInput struct {
	ParticipantName string
	Version         string
	DeploymentsPage int64
	ChecksPage      int64
}

type ReportedCheck struct {
	Check  model.CompatibilityCheck
	Report breakreport.Report
}

type ShowContractVersionOutput struct {
	Status             ShowContractVersionStatus
	ContractVersion    *model.ContractVersion
	CurrentDeployments []model.Deployment
	LatestChecks       []ReportedCheck
	Deployments        []model.Deployment
	DeploymentsTotal   int
	Checks             []ReportedCheck
	ChecksTotal        int
}

type ShowContractVersionUseCase struct {
	contractRepository      *repository.ContractRepository
	deploymentRepository    *repository.DeploymentRepository
	compatibilityRepository *repository.CompatibilityRepository
}

func NewShowContractVersionUseCase(
	contractRepository *repository.ContractRepository,
	deploymentRepository *repository.DeploymentRepository,
	compatibilityRepository *repository.CompatibilityRepository,
) *ShowContractVersionUseCase {
	return &ShowContractVersionUseCase{
		contractRepository:      contractRepository,
		deploymentRepository:    deploymentRepository,
		compatibilityRepository: compatibilityRepository,
	}
}

func (this *ShowContractVersionUseCase) Execute(ctx context.Context, input ShowContractVersionInput) ShowContractVersionOutput {
	contractVersion, found := this.contractRepository.FindContractVersion(ctx, input.ParticipantName, input.Version)
	if !found {
		return ShowContractVersionOutput{Status: ContractVersionNotFound}
	}

	latestChecks := this.compatibilityRepository.ListLatestChecksOfVersion(ctx, contractVersion.ParticipantID, contractVersion.Version)
	deployments, deploymentsTotal := this.deploymentRepository.ListDeploymentsOfVersion(
		ctx, contractVersion.ParticipantID, contractVersion.Version, pages.PageSize, pages.Offset(input.DeploymentsPage),
	)
	checks, checksTotal := this.compatibilityRepository.ListChecksOfVersion(
		ctx, contractVersion.ParticipantID, contractVersion.Version, pages.PageSize, pages.Offset(input.ChecksPage),
	)
	results := this.compatibilityRepository.ListResultsOfChecks(ctx, checkIDs(latestChecks, checks))

	return ShowContractVersionOutput{
		Status:             ContractVersionFound,
		ContractVersion:    contractVersion,
		CurrentDeployments: this.deploymentRepository.ListCurrentDeploymentsOfVersion(ctx, contractVersion.ParticipantID, contractVersion.Version),
		LatestChecks:       toReportedChecks(latestChecks, results),
		Deployments:        deployments,
		DeploymentsTotal:   deploymentsTotal,
		Checks:             toReportedChecks(checks, results),
		ChecksTotal:        checksTotal,
	}
}

func checkIDs(checkLists ...[]model.CompatibilityCheck) []int64 {
	var ids []int64
	for _, checks := range checkLists {
		for _, check := range checks {
			ids = append(ids, check.ID)
		}
	}

	return ids
}

func toReportedChecks(checks []model.CompatibilityCheck, results map[int64][]model.CompatibilityCheckResult) []ReportedCheck {
	reported := make([]ReportedCheck, 0, len(checks))
	for _, check := range checks {
		counterparts := make([]breakreport.StoredCounterpart, 0, len(results[check.ID]))
		for _, result := range results[check.ID] {
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

		reported = append(reported, ReportedCheck{Check: check, Report: report})
	}

	return reported
}
