package rename_participant

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

//go:embed rename_participant.html
var pageFiles embed.FS

type RenameParticipantUiHandler struct {
	useCase       *RenameParticipantUseCase
	sessionKeeper *sessions.Keeper
}

func NewRenameParticipantUiHandler(useCase *RenameParticipantUseCase, sessionKeeper *sessions.Keeper) *RenameParticipantUiHandler {
	return &RenameParticipantUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *RenameParticipantUiHandler) ShowForm(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	name := ctx.Params(participantNameParam)
	if validations.ParticipantName(name) != nil {
		return respond.NotFound(ctx)
	}

	target := this.useCase.FindTarget(ctx.Context(), name)
	if target.Status == ParticipantNotFound {
		return respond.NotFound(ctx)
	}

	page := newRenameParticipantPage(name, name, target.Status == ParticipantHasPublishedContracts)

	return pages.Render(ctx, fiber.StatusOK, pageFiles, renameParticipantTemplate, page)
}

func (this *RenameParticipantUiHandler) RenameParticipant(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	name := ctx.Params(participantNameParam)
	if validations.ParticipantName(name) != nil {
		return respond.NotFound(ctx)
	}

	target := this.useCase.FindTarget(ctx.Context(), name)
	if target.Status == ParticipantNotFound {
		return respond.NotFound(ctx)
	}

	newName := ctx.FormValue(nameFormField)
	output := this.useCase.Execute(ctx.Context(), RenameParticipantInput{OldName: name, NewName: newName})

	page := newRenameParticipantPage(name, newName, target.Status == ParticipantHasPublishedContracts)

	switch output.Status {
	case ParticipantRenamed:
		this.sessionKeeper.SetMessage(ctx, ParticipantRenamedMessage)
		return pages.Redirect(ctx, "/participants/"+newName)
	case ParticipantNotFound:
		return respond.NotFound(ctx)
	case ParticipantNameInvalid:
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, nameFormField, ParticipantNameNotSnakeCaseMessage)
	case ParticipantAlreadyExists:
		return this.respondProblem(ctx, fiber.StatusConflict, page, nameFormField, ParticipantAlreadyExistsMessage)
	}

	page = newRenameParticipantPage(name, newName, true)
	return pages.Render(ctx, fiber.StatusConflict, pageFiles, renameParticipantTemplate, page)
}

func (this *RenameParticipantUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *RenameParticipantUiHandler) respondProblem(ctx fiber.Ctx, status int, page RenameParticipantPage, field string, message string) error {
	page.Field = field
	page.Message = message
	return pages.Render(ctx, status, pageFiles, renameParticipantTemplate, page)
}
