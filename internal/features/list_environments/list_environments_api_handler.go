package list_environments

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type ListEnvironmentsApiHandler struct {
	useCase *ListEnvironmentsUseCase
}

func NewListEnvironmentsApiHandler(useCase *ListEnvironmentsUseCase) *ListEnvironmentsApiHandler {
	return &ListEnvironmentsApiHandler{useCase: useCase}
}

func (this *ListEnvironmentsApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return this.respondForbidden(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListEnvironmentsInput{})

	return ctx.Status(fiber.StatusOK).JSON(ListEnvironmentsResponseBody{
		Message:      EnvironmentsListedMessage,
		Environments: output.Environments,
	})
}

func (this *ListEnvironmentsApiHandler) respondForbidden(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusForbidden).JSON(ListEnvironmentsErrorResponseBody{
		Message: access.ForbiddenMessage,
	})
}
