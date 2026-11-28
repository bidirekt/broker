package publish_contract

import (
	"encoding/json"
	"errors"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type PublishContractApiHandler struct {
	useCase *PublishContractUseCase
}

func NewPublishContractApiHandler(useCase *PublishContractUseCase) *PublishContractApiHandler {
	return &PublishContractApiHandler{useCase: useCase}
}

func (this *PublishContractApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	requestBody := &PublishContractRequestBody{}
	if err := json.Unmarshal(ctx.Body(), requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	versionErr := validations.Version(requestBody.Version)
	if validations.UnpaddedValue(requestBody.ServiceName) != nil || errors.Is(versionErr, validations.ErrEmptyVersion) || len(requestBody.Contracts) == 0 || contractfiles.HasBlankSource(requestBody.Contracts) {
		return this.respondInvalidInput(ctx)
	}

	if versionErr != nil {
		return this.respondInvalidVersion(ctx, versionErr)
	}

	output, err := this.useCase.Execute(ctx.Context(), PublishContractInput{
		ServiceName: requestBody.ServiceName,
		Version:     requestBody.Version,
		Contracts:   requestBody.Contracts,
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case ContractParticipantNotFound:
		return this.respondParticipantNotFound(ctx)
	case ContractFilesUnreadable:
		return this.respondBadRequest(ctx, output.ReadError)
	case ContractValidationFailed:
		return this.respondValidationFailed(ctx, output.Violations)
	case ContractVersionConflict:
		return this.respondVersionConflict(ctx)
	}

	return this.respondSuccess(ctx)
}

func (this *PublishContractApiHandler) respondForbidden(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusForbidden).JSON(PublishContractResponseBody{
		Message: access.ForbiddenMessage,
	})
}

func (this *PublishContractApiHandler) respondParticipantNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(PublishContractResponseBody{
		Message: ContractParticipantNotFoundMessage,
	})
}

func (this *PublishContractApiHandler) respondBadRequest(ctx fiber.Ctx, message string) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractResponseBody{
		Message: message,
	})
}

func (this *PublishContractApiHandler) respondValidationFailed(ctx fiber.Ctx, violations []violation.Violation) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractValidationResponseBody{
		Message:    ContractValidationFailedMessage,
		Violations: violations,
	})
}

func (this *PublishContractApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractResponseBody{
		Message: ContractInvalidInputMessage,
	})
}

func (this *PublishContractApiHandler) respondInvalidVersion(ctx fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractResponseBody{
		Message: err.Error(),
	})
}

func (this *PublishContractApiHandler) respondVersionConflict(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusConflict).JSON(PublishContractResponseBody{
		Message: ContractVersionConflictMessage,
	})
}

func (this *PublishContractApiHandler) respondSuccess(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(PublishContractResponseBody{
		Message: ContractPublishSuccessfulMessage,
	})
}
