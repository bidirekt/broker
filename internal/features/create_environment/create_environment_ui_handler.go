package create_environment

import (
	"embed"
	"strconv"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

//go:embed create_environment.html
var pageFiles embed.FS

type CreateEnvironmentUiHandler struct {
	useCase       *CreateEnvironmentUseCase
	sessionKeeper *sessions.Keeper
}

func NewCreateEnvironmentUiHandler(useCase *CreateEnvironmentUseCase, sessionKeeper *sessions.Keeper) *CreateEnvironmentUiHandler {
	return &CreateEnvironmentUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *CreateEnvironmentUiHandler) ShowForm(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, createEnvironmentTemplate, CreateEnvironmentPage{})
}

func (this *CreateEnvironmentUiHandler) Create(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionWrite) {
		return this.respondForbidden(ctx)
	}

	page := CreateEnvironmentPage{Name: ctx.FormValue(nameFormField)}
	if validations.UnpaddedValue(page.Name) != nil {
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, EnvironmentInvalidInputMessage)
	}

	output := this.useCase.Execute(ctx.Context(), CreateEnvironmentInput{Environment: page.Name})

	switch output.Status {
	case EnvironmentAlreadyExists:
		return this.respondProblem(ctx, fiber.StatusConflict, page, EnvironmentAlreadyExistsMessage)
	}

	this.sessionKeeper.AddMessage(ctx, model.MessageSuccess, EnvironmentCreatedMessage)
	return pages.Redirect(ctx, environmentPathPrefix+strconv.FormatInt(output.ID, 10))
}

func (this *CreateEnvironmentUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *CreateEnvironmentUiHandler) respondProblem(ctx fiber.Ctx, status int, page CreateEnvironmentPage, message string) error {
	page.Field = nameFormField
	page.Message = message
	return pages.Render(ctx, status, pageFiles, createEnvironmentTemplate, page)
}
