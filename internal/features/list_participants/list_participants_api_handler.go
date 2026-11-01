package list_participants

import "github.com/gofiber/fiber/v3"

type ListParticipantsApiHandler struct {
	useCase *ListParticipantsUseCase
}

func NewListParticipantsApiHandler(useCase *ListParticipantsUseCase) *ListParticipantsApiHandler {
	return &ListParticipantsApiHandler{useCase: useCase}
}

func (this *ListParticipantsApiHandler) Handle(ctx fiber.Ctx) error {
	output := this.useCase.Execute(ctx.Context(), ListParticipantsInput{})

	return ctx.Status(fiber.StatusOK).JSON(ListParticipantsResponseBody{
		Message:      ParticipantsListedMessage,
		Participants: output.Participants,
	})
}
