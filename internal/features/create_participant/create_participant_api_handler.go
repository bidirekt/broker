package create_participant

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type CreateParticipantApiHandler struct {
	useCase *CreateParticipantUseCase
}

func NewCreateParticipantApiHandler(useCase *CreateParticipantUseCase) *CreateParticipantApiHandler {
	return &CreateParticipantApiHandler{useCase: useCase}
}

func (this *CreateParticipantApiHandler) Handle(ctx fiber.Ctx) error {
	if err := access.RequirePermission(ctx, model.PermissionWrite); err != nil {
		return err
	}

	requestBody := &CreateParticipantRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.Participant == "" {
		return this.respondInvalidInput(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), CreateParticipantInput{
		Participant: requestBody.Participant,
	})

	switch output.Status {
	case ParticipantNameInvalid:
		return this.respondInvalidName(ctx)
	case ParticipantAlreadyExists:
		return this.respondAlreadyExists(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(CreateParticipantResponseBody{
		Message: ParticipantCreatedMessage,
	})
}

func (this *CreateParticipantApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CreateParticipantResponseBody{
		Message: ParticipantInvalidInputMessage,
	})
}

func (this *CreateParticipantApiHandler) respondInvalidName(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CreateParticipantResponseBody{
		Message: ParticipantNameNotSnakeCaseMessage,
	})
}

func (this *CreateParticipantApiHandler) respondAlreadyExists(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(CreateParticipantResponseBody{
		Message: ParticipantAlreadyExistsMessage,
	})
}
