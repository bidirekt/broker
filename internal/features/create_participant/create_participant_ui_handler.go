package create_participant

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed create_participant.html
var pageFiles embed.FS

type CreateParticipantUiHandler struct {
	useCase       *CreateParticipantUseCase
	sessionKeeper *sessions.Keeper
}

func NewCreateParticipantUiHandler(useCase *CreateParticipantUseCase, sessionKeeper *sessions.Keeper) *CreateParticipantUiHandler {
	return &CreateParticipantUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *CreateParticipantUiHandler) ShowForm(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, createParticipantTemplate, CreateParticipantPage{})
}

func (this *CreateParticipantUiHandler) Create(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	page := CreateParticipantPage{Name: ctx.FormValue(nameFormField)}
	if page.Name == "" {
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, ParticipantInvalidInputMessage)
	}

	output := this.useCase.Execute(ctx.Context(), CreateParticipantInput{Participant: page.Name})

	switch output.Status {
	case ParticipantNameInvalid:
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, ParticipantNameNotSnakeCaseMessage)
	case ParticipantAlreadyExists:
		return this.respondProblem(ctx, fiber.StatusConflict, page, ParticipantAlreadyExistsMessage)
	}

	this.sessionKeeper.SetMessage(ctx, ParticipantCreatedMessage)
	return pages.Redirect(ctx, participantPathPrefix+page.Name)
}

func (this *CreateParticipantUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *CreateParticipantUiHandler) respondProblem(ctx fiber.Ctx, status int, page CreateParticipantPage, message string) error {
	page.Field = nameFormField
	page.Message = message
	return pages.Render(ctx, status, pageFiles, createParticipantTemplate, page)
}
