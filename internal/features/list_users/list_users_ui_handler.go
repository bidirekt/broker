package list_users

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
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
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListUsersInput{Page: page})

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Total)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, listUsersTemplate, ListUsersPage{Users: output.Users, Pager: pager})
}

func (this *ListUsersUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}
