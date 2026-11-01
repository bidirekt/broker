package invite_user

import (
	"embed"

	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

//go:embed invite_user.html
var pageFiles embed.FS

type InviteUserUiHandler struct {
	useCase *InviteUserUseCase
}

func NewInviteUserUiHandler(useCase *InviteUserUseCase) *InviteUserUiHandler {
	return &InviteUserUiHandler{useCase: useCase}
}

func (this *InviteUserUiHandler) ShowForm(ctx fiber.Ctx) error {
	if err := components.RequireAdmin(ctx); err != nil {
		return err
	}

	return components.Render(ctx, fiber.StatusOK, pageFiles, inviteUserTemplate, InviteUserPage{})
}

func (this *InviteUserUiHandler) Invite(ctx fiber.Ctx) error {
	if err := components.RequireAdmin(ctx); err != nil {
		return err
	}

	page := InviteUserPage{
		Email:      ctx.FormValue(emailFormField),
		Permission: ctx.FormValue(permissionFormField),
	}

	output, err := this.useCase.Execute(ctx.Context(), InviteUserInput{
		Email:      page.Email,
		Permission: page.Permission,
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case EmailInvalid:
		return this.respondWithMessage(ctx, fiber.StatusBadRequest, page, EmailInvalidMessage)
	case PermissionInvalid:
		return this.respondWithMessage(ctx, fiber.StatusBadRequest, page, PermissionInvalidMessage)
	case EmailAlreadyInUse:
		return this.respondWithMessage(ctx, fiber.StatusConflict, page, EmailAlreadyInUseMessage)
	}

	page.Link = output.Link
	return components.Render(ctx, fiber.StatusOK, pageFiles, inviteUserTemplate, page)
}

func (this *InviteUserUiHandler) respondWithMessage(ctx fiber.Ctx, status int, page InviteUserPage, message string) error {
	page.Message = message
	return components.Render(ctx, status, pageFiles, inviteUserTemplate, page)
}
