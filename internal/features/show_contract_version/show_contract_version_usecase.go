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
	contractVersionPageRepository *repository.ContractVersionPageRepository
	reportReader                  *breakreport.Reader
}

func NewShowContractVersionUseCase(
	contractVersionPageRepository *repository.ContractVersionPageRepository,
	reportReader *breakreport.Reader,
) *ShowContractVersionUseCase {
	return &ShowContractVersionUseCase{contractVersionPageRepository: contractVersionPageRepository, reportReader: reportReader}
}

func (this *ShowContractVersionUseCase) Execute(ctx context.Context, input ShowContractVersionInput) ShowContractVersionOutput {
	contractVersion, found := this.contractVersionPageRepository.FindContractVersion(ctx, input.ParticipantName, input.Version)
	if !found {
		return ShowContractVersionOutput{Status: ContractVersionNotFound}
	}

	latestChecks := this.contractVersionPageRepository.ListLatestChecksOfVersion(ctx, contractVersion.ParticipantID, contractVersion.Version)
	deployments, deploymentsTotal := this.contractVersionPageRepository.ListDeploymentsOfVersion(
		ctx, contractVersion.ParticipantID, contractVersion.Version, pages.PageSize, pages.Offset(input.DeploymentsPage),
	)
	checks, checksTotal := this.contractVersionPageRepository.ListChecksOfVersion(
		ctx, contractVersion.ParticipantID, contractVersion.Version, pages.PageSize, pages.Offset(input.ChecksPage),
	)
	results := this.contractVersionPageRepository.ListResultsOfChecks(ctx, checkIDs(latestChecks, checks))

	return ShowContractVersionOutput{
		Status:             ContractVersionFound,
		ContractVersion:    contractVersion,
		CurrentDeployments: this.contractVersionPageRepository.ListCurrentDeploymentsOfVersion(ctx, contractVersion.ParticipantID, contractVersion.Version),
		LatestChecks:       this.toReportedChecks(latestChecks, results),
		Deployments:        deployments,
		DeploymentsTotal:   deploymentsTotal,
		Checks:             this.toReportedChecks(checks, results),
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

func (this *ShowContractVersionUseCase) toReportedChecks(checks []model.CompatibilityCheck, results map[int64][]model.CompatibilityCheckResult) []ReportedCheck {
	reported := make([]ReportedCheck, 0, len(checks))
	for _, check := range checks {
		reported = append(reported, ReportedCheck{Check: check, Report: this.reportReader.FromStoredCheck(check, results[check.ID])})
	}

	return reported
}
