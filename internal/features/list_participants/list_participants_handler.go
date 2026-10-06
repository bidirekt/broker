package list_participants

import (
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

type ListParticipantsHandler struct {
	participantRepository *repository.ParticipantRepository
}

func NewListParticipantsHandler(repo *repository.ParticipantRepository) *ListParticipantsHandler {
	return &ListParticipantsHandler{participantRepository: repo}
}

func (this *ListParticipantsHandler) Handle(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(ListParticipantsResponseBody{
		Message:      ParticipantsListed,
		Participants: this.participantRepository.ListNames(ctx.Context()),
	})
}
