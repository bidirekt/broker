package record_deployment

import (
	"errors"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type RecordDeploymentApiHandler struct {
	useCase *RecordDeploymentUseCase
}

func NewRecordDeploymentApiHandler(useCase *RecordDeploymentUseCase) *RecordDeploymentApiHandler {
	return &RecordDeploymentApiHandler{useCase: useCase}
}

func (this *RecordDeploymentApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	requestBody := &RecordDeploymentRequestBody{}
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

	output := this.useCase.Execute(ctx.Context(), RecordDeploymentInput{
		Participant: requestBody.Participant,
		Version:     requestBody.Version,
		Environment: requestBody.Environment,
	})

	switch output.Status {
	case ParticipantNotFound:
		return this.respondParticipantNotFound(ctx)
	case VersionNotFound:
		return this.respondVersionNotFound(ctx)
	case EnvironmentNotFound:
		return this.respondEnvironmentNotFound(ctx)
	case DeploymentRolledBack:
		return this.respondRolledBack(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(RecordDeploymentResponseBody{
		Message: DeploymentRecordedMessage,
	})
}

func (this *RecordDeploymentApiHandler) respondForbidden(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusForbidden).JSON(RecordDeploymentResponseBody{
		Message: access.ForbiddenMessage,
	})
}

func (this *RecordDeploymentApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RecordDeploymentResponseBody{
		Message: DeploymentInvalidInputMessage,
	})
}

func (this *RecordDeploymentApiHandler) respondInvalidVersion(ctx fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RecordDeploymentResponseBody{
		Message: err.Error(),
	})
}

func (this *RecordDeploymentApiHandler) respondEnvironmentNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RecordDeploymentResponseBody{
		Message: EnvironmentNotFoundMessage,
	})
}

func (this *RecordDeploymentApiHandler) respondParticipantNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RecordDeploymentResponseBody{
		Message: ParticipantNotFoundMessage,
	})
}

func (this *RecordDeploymentApiHandler) respondVersionNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RecordDeploymentResponseBody{
		Message: VersionNotFoundMessage,
	})
}

func (this *RecordDeploymentApiHandler) respondRolledBack(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(RecordDeploymentResponseBody{
		Message: DeploymentRolledBackMessage,
	})
}
