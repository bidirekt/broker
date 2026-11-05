package show_check

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_check.html
var pageFiles embed.FS

type ShowCheckUiHandler struct {
	useCase *ShowCheckUseCase
}

func NewShowCheckUiHandler(useCase *ShowCheckUseCase) *ShowCheckUiHandler {
	return &ShowCheckUiHandler{useCase: useCase}
}

func (this *ShowCheckUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	checkID, ok := pages.ParsePositiveInteger(ctx.Params(checkIDParam))
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowCheckInput{CheckID: checkID})
	if output.Status == CheckNotFound {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showCheckTemplate, newShowCheckPage(output.Check, output.Report))
}
