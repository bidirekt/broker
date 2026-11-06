package list_participant_pages

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed list_participant_pages.html
var pageFiles embed.FS

type ListParticipantPagesUiHandler struct {
	useCase *ListParticipantPagesUseCase
}

func NewListParticipantPagesUiHandler(useCase *ListParticipantPagesUseCase) *ListParticipantPagesUiHandler {
	return &ListParticipantPagesUiHandler{useCase: useCase}
}

func (this *ListParticipantPagesUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListParticipantPagesInput{Page: page})

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Total)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, listParticipantPagesTemplate, newListParticipantPagesPage(output.Participants, pager))
}
