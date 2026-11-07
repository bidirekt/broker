package rename_environment

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type RenameEnvironmentApiHandler struct {
	useCase *RenameEnvironmentUseCase
}

func NewRenameEnvironmentApiHandler(useCase *RenameEnvironmentUseCase) *RenameEnvironmentApiHandler {
	return &RenameEnvironmentApiHandler{useCase: useCase}
}

func (this *RenameEnvironmentApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respond(ctx, fiber.StatusForbidden, access.ForbiddenMessage)
	}

	requestBody := &RenameEnvironmentRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respond(ctx, fiber.StatusBadRequest, EnvironmentInvalidInputMessage)
	}

	output := this.useCase.RenameByName(ctx.Context(), RenameEnvironmentByNameInput{
		OldName: requestBody.OldName,
		NewName: requestBody.NewName,
	})

	switch output.Status {
	case EnvironmentInvalidInput:
		return this.respond(ctx, fiber.StatusBadRequest, EnvironmentInvalidInputMessage)
	case EnvironmentNotFound:
		return this.respond(ctx, fiber.StatusNotFound, EnvironmentNotFoundMessage)
	case EnvironmentAlreadyExists:
		return this.respond(ctx, fiber.StatusConflict, EnvironmentAlreadyExistsMessage)
	}

	return this.respond(ctx, fiber.StatusOK, EnvironmentRenamedMessage)
}

func (this *RenameEnvironmentApiHandler) respond(ctx fiber.Ctx, status int, message string) error {
	return ctx.Status(status).JSON(RenameEnvironmentResponseBody{Message: message})
}
