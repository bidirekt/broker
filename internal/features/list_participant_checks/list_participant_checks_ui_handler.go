package list_participant_checks

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

//go:embed list_participant_checks.html
var pageFiles embed.FS

type ListParticipantChecksUiHandler struct {
	useCase *ListParticipantChecksUseCase
}

func NewListParticipantChecksUiHandler(useCase *ListParticipantChecksUseCase) *ListParticipantChecksUiHandler {
	return &ListParticipantChecksUiHandler{useCase: useCase}
}

func (this *ListParticipantChecksUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	name := ctx.Params(nameParam)
	if validations.ParticipantName(name) != nil {
		return respond.NotFound(ctx)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListParticipantChecksInput{Name: name, Page: page})
	if output.Status == ParticipantNotFound {
		return respond.NotFound(ctx)
	}

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Header.Checks)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, listParticipantChecksTemplate, newListParticipantChecksPage(output, pager))
}
