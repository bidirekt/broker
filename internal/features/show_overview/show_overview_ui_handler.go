package show_overview

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_overview.html
var pageFiles embed.FS

type ShowOverviewUiHandler struct {
	useCase *ShowOverviewUseCase
}

func NewShowOverviewUiHandler(useCase *ShowOverviewUseCase) *ShowOverviewUiHandler {
	return &ShowOverviewUiHandler{useCase: useCase}
}

func (this *ShowOverviewUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	output := this.useCase.Execute(ctx.Context(), ShowOverviewInput{})

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showOverviewTemplate, newShowOverviewPage(output))
}
