package validate_contract

import (
	"encoding/json"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type ValidateContractApiHandler struct {
	useCase *ValidateContractUseCase
}

func NewValidateContractApiHandler(useCase *ValidateContractUseCase) *ValidateContractApiHandler {
	return &ValidateContractApiHandler{useCase: useCase}
}

func (this *ValidateContractApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return this.respondForbidden(ctx)
	}

	requestBody := &ValidateContractRequestBody{}
	if err := json.Unmarshal(ctx.Body(), requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if validations.UnpaddedValue(requestBody.Participant) != nil || validations.UnpaddedValue(requestBody.Environment) != nil || len(requestBody.Contracts) == 0 || contractfiles.HasBlankSource(requestBody.Contracts) {
		return this.respondInvalidInput(ctx)
	}

	output, err := this.useCase.Execute(ctx.Context(), ValidateContractInput{
		ParticipantName: requestBody.Participant,
		EnvironmentName: requestBody.Environment,
		Contracts:       requestBody.Contracts,
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case ParticipantNotFound:
		return this.respondNotFound(ctx, ParticipantNotFoundMessage)
	case EnvironmentNotFound:
		return this.respondNotFound(ctx, EnvironmentNotFoundMessage)
	case ContractFilesUnreadable:
		return this.respondBadRequest(ctx, output.ReadError)
	case ContractValidationFailed:
		return this.respondValidationFailed(ctx, output.Violations)
	}

	return ctx.Status(fiber.StatusOK).JSON(ValidateContractResponseBody{
		Message:     ContractValidatedMessage,
		Participant: output.Participant,
		Environment: output.Environment,
		Deployable:  output.Deployable,
		Results:     output.Results,
	})
}

func (this *ValidateContractApiHandler) respondForbidden(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusForbidden).JSON(ValidateContractErrorResponseBody{
		Message: access.ForbiddenMessage,
	})
}

func (this *ValidateContractApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(ValidateContractErrorResponseBody{
		Message: ContractInvalidInputMessage,
	})
}

func (this *ValidateContractApiHandler) respondBadRequest(ctx fiber.Ctx, message string) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(ValidateContractErrorResponseBody{
		Message: message,
	})
}

func (this *ValidateContractApiHandler) respondValidationFailed(ctx fiber.Ctx, violations []violation.Violation) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(ValidateContractValidationResponseBody{
		Message:    ContractValidationFailedMessage,
		Violations: violations,
	})
}

func (this *ValidateContractApiHandler) respondNotFound(ctx fiber.Ctx, message string) error {
	return ctx.Status(fiber.StatusNotFound).JSON(ValidateContractErrorResponseBody{
		Message: message,
	})
}
