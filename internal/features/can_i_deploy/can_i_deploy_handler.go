package can_i_deploy

import (
	"context"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/guregu/null"
)

type CanIDeployHandler struct {
	contractRepository      *repository.ContractRepository
	environmentRepository   *repository.EnvironmentRepository
	compatibilityRepository *repository.CompatibilityRepository
	compatibilityChecker    *compatibility_checker.CompatibilityChecker
	participantRepository   *repository.ParticipantRepository
}

func NewCanIDeployHandler(
	contractRepository *repository.ContractRepository,
	environmentRepository *repository.EnvironmentRepository,
	compatibilityRepository *repository.CompatibilityRepository,
	compatibilityChecker *compatibility_checker.CompatibilityChecker,
	participantRepository *repository.ParticipantRepository,
) *CanIDeployHandler {
	return &CanIDeployHandler{
		contractRepository:      contractRepository,
		environmentRepository:   environmentRepository,
		compatibilityRepository: compatibilityRepository,
		compatibilityChecker:    compatibilityChecker,
		participantRepository:   participantRepository,
	}
}

func (this *CanIDeployHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &CanIDeployRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.Participant == "" || requestBody.Version == "" || requestBody.Environment == "" {
		return this.respondInvalidInput(ctx)
	}

	participant, exists := this.participantRepository.FindByName(ctx.Context(), requestBody.Participant)
	if !exists {
		return this.respondParticipantNotFound(ctx)
	}

	contract, exists := this.contractRepository.GetContractByNameAndVersion(
		ctx.Context(),
		participant.Name,
		requestBody.Version,
	)

	if !exists {
		return this.respondContractNotFound(ctx)
	}

	environment, exists := this.environmentRepository.FindByName(ctx.Context(), requestBody.Environment)
	if !exists {
		return this.respondEnvironmentNotFound(ctx)
	}

	counterparts := this.contractRepository.LoadCounterparts(ctx.Context(), contract, environment.ID)

	compatibilityReport := this.compatibilityChecker.Check(
		ctx.Context(),
		contract,
		environment,
		counterparts,
	)

	deployable := true
	for _, result := range compatibilityReport.Results {
		deployable = deployable && result.Deployable
	}

	this.recordCheck(ctx.Context(), contract, environment, deployable, compatibilityReport)

	return ctx.Status(fiber.StatusOK).JSON(CanIDeployResponseBody{
		Message:     ContractChecked,
		Participant: requestBody.Participant,
		Version:     requestBody.Version,
		Environment: requestBody.Environment,
		Deployable:  deployable,
		Results:     compatibilityReport.Hierarchical,
	})
}

func (this *CanIDeployHandler) recordCheck(
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

func (this *CanIDeployHandler) respondParticipantNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(CanIDeployErrorResponseBody{
		Message: ParticipantNotFound,
	})
}

func (this *CanIDeployHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CanIDeployErrorResponseBody{
		Message: CanIDeployInvalidInput,
	})
}

func (this *CanIDeployHandler) respondContractNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(CanIDeployErrorResponseBody{
		Message: ContractNotFound,
	})
}

func (this *CanIDeployHandler) respondEnvironmentNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(CanIDeployErrorResponseBody{
		Message: EnvironmentNotFound,
	})
}
