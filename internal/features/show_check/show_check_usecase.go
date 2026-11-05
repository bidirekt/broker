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
}

func NewShowCheckUseCase(compatibilityRepository *repository.CompatibilityRepository) *ShowCheckUseCase {
	return &ShowCheckUseCase{compatibilityRepository: compatibilityRepository}
}

func (this *ShowCheckUseCase) Execute(ctx context.Context, input ShowCheckInput) ShowCheckOutput {
	check, found := this.compatibilityRepository.FindCheck(ctx, input.CheckID)
	if !found {
		return ShowCheckOutput{Status: CheckNotFound}
	}

	results := this.compatibilityRepository.ListCheckResults(ctx, check.ID)
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

	return ShowCheckOutput{Status: CheckFound, Check: check, Report: report}
}
