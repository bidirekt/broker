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
	if err := access.RequirePermission(ctx, model.PermissionRead); err != nil {
		return err
	}

	output := this.useCase.Execute(ctx.Context(), ListParticipantsInput{})

	return ctx.Status(fiber.StatusOK).JSON(ListParticipantsResponseBody{
		Message:      ParticipantsListedMessage,
		Participants: output.Participants,
	})
}
