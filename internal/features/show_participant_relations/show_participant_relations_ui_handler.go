package show_participant_relations

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_participant_relations.html
var pageFiles embed.FS

type ShowParticipantRelationsUiHandler struct {
	useCase *ShowParticipantRelationsUseCase
}

func NewShowParticipantRelationsUiHandler(useCase *ShowParticipantRelationsUseCase) *ShowParticipantRelationsUiHandler {
	return &ShowParticipantRelationsUiHandler{useCase: useCase}
}

func (this *ShowParticipantRelationsUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	name := ctx.Params(participantNameParam)
	if validations.ParticipantName(name) != nil {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowParticipantRelationsInput{Name: name})
	if output.Status == ParticipantNotFound {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showParticipantRelationsTemplate, newShowParticipantRelationsPage(output))
}
