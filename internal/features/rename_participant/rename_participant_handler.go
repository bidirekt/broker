package rename_participant

import (
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type RenameParticipantHandler struct {
	participantRepository *repository.ParticipantRepository
}

func NewRenameParticipantHandler(repo *repository.ParticipantRepository) *RenameParticipantHandler {
	return &RenameParticipantHandler{participantRepository: repo}
}

func (this *RenameParticipantHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &RenameParticipantRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.OldName == "" || requestBody.NewName == "" {
		return this.respondInvalidInput(ctx)
	}

	// only the new name is judged: a legacy participant whose name predates the
	// spelling rule must still be renameable into a valid one
	if validations.ParticipantName(requestBody.NewName) != nil {
		return this.respondInvalidName(ctx)
	}

	found, conflict, published := this.participantRepository.Rename(ctx.Context(), requestBody.OldName, requestBody.NewName)
	if conflict {
		return this.respondAlreadyExists(ctx)
	}

	if !found {
		return this.respondNotFound(ctx)
	}

	if published {
		return this.respondHasPublishedContracts(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(RenameParticipantResponseBody{
		Message: ParticipantRenamed,
	})
}

func (this *RenameParticipantHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RenameParticipantResponseBody{
		Message: ParticipantInvalidInput,
	})
}

func (this *RenameParticipantHandler) respondInvalidName(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RenameParticipantResponseBody{
		Message: ParticipantNameNotSnakeCase,
	})
}

func (this *RenameParticipantHandler) respondAlreadyExists(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusConflict).JSON(RenameParticipantResponseBody{
		Message: ParticipantAlreadyExists,
	})
}

func (this *RenameParticipantHandler) respondNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RenameParticipantResponseBody{
		Message: ParticipantNotFound,
	})
}

func (this *RenameParticipantHandler) respondHasPublishedContracts(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusConflict).JSON(RenameParticipantResponseBody{
		Message: ParticipantHasPublishedContracts,
	})
}
