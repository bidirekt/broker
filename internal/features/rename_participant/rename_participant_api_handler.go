package rename_participant

import "github.com/gofiber/fiber/v3"

type RenameParticipantApiHandler struct {
	useCase *RenameParticipantUseCase
}

func NewRenameParticipantApiHandler(useCase *RenameParticipantUseCase) *RenameParticipantApiHandler {
	return &RenameParticipantApiHandler{useCase: useCase}
}

func (this *RenameParticipantApiHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &RenameParticipantRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.OldName == "" || requestBody.NewName == "" {
		return this.respondInvalidInput(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), RenameParticipantInput{
		OldName: requestBody.OldName,
		NewName: requestBody.NewName,
	})

	switch output.Status {
	case ParticipantNameInvalid:
		return this.respondInvalidName(ctx)
	case ParticipantAlreadyExists:
		return this.respondAlreadyExists(ctx)
	case ParticipantNotFound:
		return this.respondNotFound(ctx)
	case ParticipantHasPublishedContracts:
		return this.respondHasPublishedContracts(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(RenameParticipantResponseBody{
		Message: ParticipantRenamedMessage,
	})
}

func (this *RenameParticipantApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RenameParticipantResponseBody{
		Message: ParticipantInvalidInputMessage,
	})
}

func (this *RenameParticipantApiHandler) respondInvalidName(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RenameParticipantResponseBody{
		Message: ParticipantNameNotSnakeCaseMessage,
	})
}

func (this *RenameParticipantApiHandler) respondAlreadyExists(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusConflict).JSON(RenameParticipantResponseBody{
		Message: ParticipantAlreadyExistsMessage,
	})
}

func (this *RenameParticipantApiHandler) respondNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RenameParticipantResponseBody{
		Message: ParticipantNotFoundMessage,
	})
}

func (this *RenameParticipantApiHandler) respondHasPublishedContracts(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusConflict).JSON(RenameParticipantResponseBody{
		Message: ParticipantHasPublishedContractsMessage,
	})
}
