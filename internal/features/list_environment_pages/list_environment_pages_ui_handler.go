package list_environment_pages

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed list_environment_pages.html
var pageFiles embed.FS

type ListEnvironmentPagesUiHandler struct {
	useCase *ListEnvironmentPagesUseCase
}

func NewListEnvironmentPagesUiHandler(useCase *ListEnvironmentPagesUseCase) *ListEnvironmentPagesUiHandler {
	return &ListEnvironmentPagesUiHandler{useCase: useCase}
}

func (this *ListEnvironmentPagesUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListEnvironmentPagesInput{Page: page})

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Total)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, listEnvironmentPagesTemplate, newListEnvironmentPagesPage(output, pager))
}
