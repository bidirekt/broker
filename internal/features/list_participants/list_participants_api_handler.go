package list_participants

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type ListParticipantsApiHandler struct {
	useCase *ListParticipantsUseCase
}

func NewListParticipantsApiHandler(useCase *ListParticipantsUseCase) *ListParticipantsApiHandler {
	return &ListParticipantsApiHandler{useCase: useCase}
}

func (this *ListParticipantsApiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return this.respondForbidden(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListParticipantsInput{})

	return ctx.Status(fiber.StatusOK).JSON(ListParticipantsResponseBody{
		Message:      ParticipantsListedMessage,
		Participants: output.Participants,
	})
}

func (this *ListParticipantsApiHandler) respondForbidden(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusForbidden).JSON(ListParticipantsErrorResponseBody{
		Message: access.ForbiddenMessage,
	})
}
