package show_check

import (
	"context"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ShowCheckStatus int

const (
	CheckFound ShowCheckStatus = iota
	CheckNotFound
)

type ShowCheckInput struct {
	CheckID int64
}

type ShowCheckOutput struct {
	Status ShowCheckStatus
	Check  *model.CompatibilityCheck
	Report breakreport.Report
}

type ShowCheckUseCase struct {
	compatibilityRepository *repository.CompatibilityRepository
	reportReader            *breakreport.Reader
}

func NewShowCheckUseCase(compatibilityRepository *repository.CompatibilityRepository, reportReader *breakreport.Reader) *ShowCheckUseCase {
	return &ShowCheckUseCase{compatibilityRepository: compatibilityRepository, reportReader: reportReader}
}

func (this *ShowCheckUseCase) Execute(ctx context.Context, input ShowCheckInput) ShowCheckOutput {
	check, found := this.compatibilityRepository.FindCheck(ctx, input.CheckID)
	if !found {
		return ShowCheckOutput{Status: CheckNotFound}
	}

	report := this.reportReader.FromStoredCheck(*check, this.compatibilityRepository.ListCheckResults(ctx, check.ID))

	return ShowCheckOutput{Status: CheckFound, Check: check, Report: report}
}
