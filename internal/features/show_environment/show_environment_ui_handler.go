package show_environment

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_environment.html
var pageFiles embed.FS

type ShowEnvironmentUiHandler struct {
	useCase *ShowEnvironmentUseCase
}

func NewShowEnvironmentUiHandler(useCase *ShowEnvironmentUseCase) *ShowEnvironmentUiHandler {
	return &ShowEnvironmentUiHandler{useCase: useCase}
}

func (this *ShowEnvironmentUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	environmentID, ok := pages.ParsePositiveInteger(ctx.Params(environmentIDParam))
	if !ok {
		return respond.NotFound(ctx)
	}

	historyPage, ok := pages.RequestedPage(ctx, historyPageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	checksPage, ok := pages.RequestedPage(ctx, checksPageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowEnvironmentInput{
		EnvironmentID: environmentID,
		HistoryPage:   historyPage,
		ChecksPage:    checksPage,
	})
	if output.Status == EnvironmentNotFound {
		return respond.NotFound(ctx)
	}

	historyPager, ok := pages.NewPager(ctx, historyPageParam, historyPage, output.DeploymentsTotal)
	if !ok {
		return respond.NotFound(ctx)
	}

	checksPager, ok := pages.NewPager(ctx, checksPageParam, checksPage, output.ChecksTotal)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showEnvironmentTemplate, newShowEnvironmentPage(output, historyPager, checksPager))
}
