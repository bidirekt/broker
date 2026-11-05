package show_participant_versions

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_participant_versions.html
var pageFiles embed.FS

type ShowParticipantVersionsUiHandler struct {
	useCase *ShowParticipantVersionsUseCase
}

func NewShowParticipantVersionsUiHandler(useCase *ShowParticipantVersionsUseCase) *ShowParticipantVersionsUiHandler {
	return &ShowParticipantVersionsUiHandler{useCase: useCase}
}

func (this *ShowParticipantVersionsUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowParticipantVersionsInput{Name: ctx.Params(participantNameParam), Page: page})
	if output.Status == ParticipantVersionsParticipantNotFound {
		return respond.NotFound(ctx)
	}

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Header.VersionCount)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showParticipantVersionsTemplate, newShowParticipantVersionsPage(output.Header, output.Versions, pager))
}
