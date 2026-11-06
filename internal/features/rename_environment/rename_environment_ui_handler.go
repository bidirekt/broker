package rename_environment

import (
	"embed"
	"strconv"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed rename_environment.html
var pageFiles embed.FS

type RenameEnvironmentUiHandler struct {
	useCase       *RenameEnvironmentUseCase
	sessionKeeper *sessions.Keeper
}

func NewRenameEnvironmentUiHandler(useCase *RenameEnvironmentUseCase, sessionKeeper *sessions.Keeper) *RenameEnvironmentUiHandler {
	return &RenameEnvironmentUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *RenameEnvironmentUiHandler) ShowForm(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	environmentID, ok := pages.ParsePositiveInteger(ctx.Params(environmentIDParam))
	if !ok {
		return respond.NotFound(ctx)
	}

	target := this.useCase.FindEnvironment(ctx.Context(), environmentID)
	if target.Status == EnvironmentNotFound {
		return respond.NotFound(ctx)
	}

	page := RenameEnvironmentPage{ID: environmentID, Name: target.Environment.Name, NewName: target.Environment.Name}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, renameEnvironmentTemplate, page)
}

func (this *RenameEnvironmentUiHandler) RenameEnvironment(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	environmentID, ok := pages.ParsePositiveInteger(ctx.Params(environmentIDParam))
	if !ok {
		return respond.NotFound(ctx)
	}

	target := this.useCase.FindEnvironment(ctx.Context(), environmentID)
	if target.Status == EnvironmentNotFound {
		return respond.NotFound(ctx)
	}

	newName := ctx.FormValue(nameFormField)
	output := this.useCase.RenameByID(ctx.Context(), RenameEnvironmentByIDInput{ID: environmentID, NewName: newName})

	page := RenameEnvironmentPage{ID: environmentID, Name: target.Environment.Name, NewName: newName}

	switch output.Status {
	case EnvironmentNotFound:
		return respond.NotFound(ctx)
	case EnvironmentInvalidInput:
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, EnvironmentInvalidInputMessage)
	case EnvironmentAlreadyExists:
		return this.respondProblem(ctx, fiber.StatusConflict, page, EnvironmentAlreadyExistsMessage)
	}

	this.sessionKeeper.SetMessage(ctx, EnvironmentRenamedMessage)
	return pages.Redirect(ctx, "/environments/"+strconv.FormatInt(environmentID, 10))
}

func (this *RenameEnvironmentUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *RenameEnvironmentUiHandler) respondProblem(ctx fiber.Ctx, status int, page RenameEnvironmentPage, message string) error {
	page.Field = nameFormField
	page.Message = message
	return pages.Render(ctx, status, pageFiles, renameEnvironmentTemplate, page)
}
