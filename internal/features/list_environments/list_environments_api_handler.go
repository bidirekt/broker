package list_environments

import (
	"github.com/bidirekt/broker/internal/components"
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
	if err := components.RequirePermission(ctx, model.PermissionRead); err != nil {
		return err
	}

	output := this.useCase.Execute(ctx.Context(), ListEnvironmentsInput{})

	return ctx.Status(fiber.StatusOK).JSON(ListEnvironmentsResponseBody{
		Message:      EnvironmentsListedMessage,
		Environments: output.Environments,
	})
}
