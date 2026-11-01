package create_participant

import (
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type CreateParticipantHandler struct {
	participantRepository *repository.ParticipantRepository
}

func NewCreateParticipantHandler(
	participantRepository *repository.ParticipantRepository,
) *CreateParticipantHandler {
	return &CreateParticipantHandler{
		participantRepository: participantRepository,
	}
}

func (this *CreateParticipantHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &CreateParticipantRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.Participant == "" {
		return this.respondInvalidInput(ctx)
	}

	if validations.ParticipantName(requestBody.Participant) != nil {
		return this.respondInvalidName(ctx)
	}

	created := this.participantRepository.Create(ctx.Context(), model.NewParticipant(requestBody.Participant))
	if !created {
		return this.respondAlreadyExists(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(CreateParticipantResponseBody{
		Message: ParticipantCreatedMessage,
	})
}

func (this *CreateParticipantHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CreateParticipantResponseBody{
		Message: ParticipantInvalidInputMessage,
	})
}

func (this *CreateParticipantHandler) respondInvalidName(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CreateParticipantResponseBody{
		Message: ParticipantNameNotSnakeCaseMessage,
	})
}

func (this *CreateParticipantHandler) respondAlreadyExists(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(CreateParticipantResponseBody{
		Message: ParticipantAlreadyExistsMessage,
	})
}
