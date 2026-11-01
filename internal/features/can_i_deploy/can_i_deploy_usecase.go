package can_i_deploy

import (
	"context"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

type CanIDeployStatus int

const (
	ContractChecked CanIDeployStatus = iota
	ParticipantNotFound
	ContractNotFound
	EnvironmentNotFound
)

type CanIDeployInput struct {
	Participant string
	Version     string
	Environment string
}

type CanIDeployOutput struct {
	Status     CanIDeployStatus
	Deployable bool
	Results    map[string]compatibility_checker.Hierarchical
}

type CanIDeployUseCase struct {
	contractRepository      *repository.ContractRepository
	environmentRepository   *repository.EnvironmentRepository
	compatibilityRepository *repository.CompatibilityRepository
	compatibilityChecker    *compatibility_checker.CompatibilityChecker
	participantRepository   *repository.ParticipantRepository
}

func NewCanIDeployUseCase(
	contractRepository *repository.ContractRepository,
	environmentRepository *repository.EnvironmentRepository,
	compatibilityRepository *repository.CompatibilityRepository,
	compatibilityChecker *compatibility_checker.CompatibilityChecker,
	participantRepository *repository.ParticipantRepository,
) *CanIDeployUseCase {
	return &CanIDeployUseCase{
		contractRepository:      contractRepository,
		environmentRepository:   environmentRepository,
		compatibilityRepository: compatibilityRepository,
		compatibilityChecker:    compatibilityChecker,
		participantRepository:   participantRepository,
	}
}

func (this *CanIDeployUseCase) Execute(ctx context.Context, input CanIDeployInput) CanIDeployOutput {
	participant, exists := this.participantRepository.FindByName(ctx, input.Participant)
	if !exists {
		return CanIDeployOutput{Status: ParticipantNotFound}
	}

	contract, exists := this.contractRepository.GetContractByNameAndVersion(ctx, participant.Name, input.Version)
	if !exists {
		return CanIDeployOutput{Status: ContractNotFound}
	}

	environment, exists := this.environmentRepository.FindByName(ctx, input.Environment)
	if !exists {
		return CanIDeployOutput{Status: EnvironmentNotFound}
	}

	counterparts := this.contractRepository.LoadCounterparts(ctx, contract, environment.ID)

	compatibilityReport := this.compatibilityChecker.Check(ctx, contract, environment, counterparts)

	deployable := compatibilityReport.Deployable()

	this.recordCheck(ctx, contract, environment, deployable, compatibilityReport)

	return CanIDeployOutput{
		Status:     ContractChecked,
		Deployable: deployable,
		Results:    compatibilityReport.Hierarchical,
	}
}

func (this *CanIDeployUseCase) recordCheck(
	ctx context.Context,
	contract *model.PersistedContract,
	environment *model.Environment,
	deployable bool,
	report *compatibility_checker.ContractCompatibilityReport,
) {
	check := &model.CompatibilityCheck{
		ParticipantID: contract.ParticipantID,
		ContractID:    contract.ID,
		Version:       contract.Version,
		EnvironmentID: environment.ID,
		Deployable:    deployable,
	}

	results := make([]model.CompatibilityCheckResult, 0, len(report.Results))
	breaksByPair := make(map[[2]int64][]model.VerdictBreak)

	for counterpartName, item := range report.Results {
		counterpart := item.IncompatibleCounterpart

		result := model.CompatibilityCheckResult{
			CounterpartName:    counterpartName,
			CounterpartVersion: counterpart.ParticipantVersion,
			Deployable:         item.Deployable,
		}

		if counterpart.ParticipantID > 0 {
			result.CounterpartParticipantID = null.IntFrom(counterpart.ParticipantID)
		}

		if counterpart.ContractID > 0 {
			contractIDOne, contractIDTwo := model.OrderContractPair(contract.ID, counterpart.ContractID)
			pair := [2]int64{contractIDOne, contractIDTwo}

			result.VerdictContractIDOne = null.IntFrom(contractIDOne)
			result.VerdictContractIDTwo = null.IntFrom(contractIDTwo)

			// A cached verdict is already stored: only the pairs compared in this call become facts.
			if !item.VerdictCached {
				breaks, exists := breaksByPair[pair]
				if !exists {
					breaks = make([]model.VerdictBreak, 0, len(item.Breaks))
				}

				for _, breakChange := range item.Breaks {
					if !breakChange.IsPropertyBreak() {
						continue
					}

					breaks = append(breaks, model.VerdictBreak{
						Endpoint:    breakChange.CheckedResource.Endpoint,
						Method:      breakChange.CheckedResource.Method,
						Interaction: breakChange.InteractionKey(),
						Reason:      string(breakChange.Reason),
						Details:     breakChange.Details,
					})
				}

				breaksByPair[pair] = breaks
			}
		}

		results = append(results, result)
	}

	newVerdicts := make([]model.CompatibilityVerdict, 0, len(breaksByPair))
	for pair, breaks := range breaksByPair {
		newVerdicts = append(newVerdicts, model.CompatibilityVerdict{
			ContractIDOne: pair[0],
			ContractIDTwo: pair[1],
			Breaks:        breaks,
		})
	}

	this.compatibilityRepository.RecordCheck(ctx, check, results, newVerdicts)
}
