package validate_contract

import (
	"encoding/json"
	"strings"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type ValidateContractApiHandler struct {
	useCase *ValidateContractUseCase
}

func NewValidateContractApiHandler(useCase *ValidateContractUseCase) *ValidateContractApiHandler {
	return &ValidateContractApiHandler{useCase: useCase}
}

func (this *ValidateContractApiHandler) Handle(ctx fiber.Ctx) error {
	if err := components.RequirePermission(ctx, model.PermissionRead); err != nil {
		return err
	}

	requestBody := &ValidateContractRequestBody{}
	if err := json.Unmarshal(ctx.Body(), requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if !isUnpaddedValue(requestBody.Participant) || !isUnpaddedValue(requestBody.Environment) || len(requestBody.Contracts) == 0 || contractfiles.HasBlankSource(requestBody.Contracts) {
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

func isUnpaddedValue(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}
