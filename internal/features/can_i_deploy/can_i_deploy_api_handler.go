package can_i_deploy

import (
	"errors"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type CanIDeployApiHandler struct {
	useCase *CanIDeployUseCase
}

func NewCanIDeployApiHandler(useCase *CanIDeployUseCase) *CanIDeployApiHandler {
	return &CanIDeployApiHandler{useCase: useCase}
}

func (this *CanIDeployApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return this.respondForbidden(ctx)
	}

	requestBody := &CanIDeployRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	versionErr := validations.Version(requestBody.Version)
	if validations.UnpaddedValue(requestBody.Participant) != nil || errors.Is(versionErr, validations.ErrEmptyVersion) || validations.UnpaddedValue(requestBody.Environment) != nil {
		return this.respondInvalidInput(ctx)
	}

	if versionErr != nil {
		return this.respondInvalidVersion(ctx, versionErr)
	}

	output := this.useCase.Execute(ctx.Context(), CanIDeployInput{
		Participant: requestBody.Participant,
		Version:     requestBody.Version,
		Environment: requestBody.Environment,
	})

	switch output.Status {
	case ParticipantNotFound:
		return this.respondParticipantNotFound(ctx)
	case ContractNotFound:
		return this.respondContractNotFound(ctx)
	case EnvironmentNotFound:
		return this.respondEnvironmentNotFound(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(CanIDeployResponseBody{
		Message:     ContractCheckedMessage,
		Participant: requestBody.Participant,
		Version:     requestBody.Version,
		Environment: requestBody.Environment,
		Deployable:  output.Deployable,
		Results:     output.Results,
	})
}

func (this *CanIDeployApiHandler) respondForbidden(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusForbidden).JSON(CanIDeployErrorResponseBody{
		Message: access.ForbiddenMessage,
	})
}

func (this *CanIDeployApiHandler) respondParticipantNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(CanIDeployErrorResponseBody{
		Message: ParticipantNotFoundMessage,
	})
}

func (this *CanIDeployApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CanIDeployErrorResponseBody{
		Message: CanIDeployInvalidInputMessage,
	})
}

func (this *CanIDeployApiHandler) respondInvalidVersion(ctx fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CanIDeployErrorResponseBody{
		Message: err.Error(),
	})
}

func (this *CanIDeployApiHandler) respondContractNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(CanIDeployErrorResponseBody{
		Message: ContractNotFoundMessage,
	})
}

func (this *CanIDeployApiHandler) respondEnvironmentNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(CanIDeployErrorResponseBody{
		Message: EnvironmentNotFoundMessage,
	})
}
