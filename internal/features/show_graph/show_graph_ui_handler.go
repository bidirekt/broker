package show_graph

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_graph.html
var pageFiles embed.FS

type ShowGraphUiHandler struct {
	useCase *ShowGraphUseCase
}

func NewShowGraphUiHandler(useCase *ShowGraphUseCase) *ShowGraphUiHandler {
	return &ShowGraphUiHandler{useCase: useCase}
}

func (this *ShowGraphUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	name := ctx.Params(graphNameParam)
	if validations.ParticipantName(name) != nil {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowGraphInput{Name: name})
	if output.Status == GraphNotFound {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showGraphTemplate, newShowGraphPage(output))
}
