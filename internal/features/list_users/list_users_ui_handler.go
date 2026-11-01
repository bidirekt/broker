package list_users

import (
	"embed"

	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

//go:embed list_users.html
var pageFiles embed.FS

type ListUsersUiHandler struct {
	useCase *ListUsersUseCase
}

func NewListUsersUiHandler(useCase *ListUsersUseCase) *ListUsersUiHandler {
	return &ListUsersUiHandler{useCase: useCase}
}

func (this *ListUsersUiHandler) Handle(ctx fiber.Ctx) error {
	if err := components.RequireAdmin(ctx); err != nil {
		return err
	}

	output := this.useCase.Execute(ctx.Context(), ListUsersInput{})

	return components.Render(ctx, fiber.StatusOK, pageFiles, listUsersTemplate, ListUsersPage{Users: output.Users})
}
