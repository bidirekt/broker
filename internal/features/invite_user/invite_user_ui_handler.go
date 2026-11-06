package invite_user

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
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
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, inviteUserTemplate, InviteUserPage{Permission: string(model.PermissionRead)})
}

func (this *InviteUserUiHandler) Invite(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
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
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, emailFormField, EmailInvalidMessage)
	case PermissionInvalid:
		page.Permission = ""
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, permissionFormField, PermissionInvalidMessage)
	case EmailAlreadyInUse:
		return this.respondProblem(ctx, fiber.StatusConflict, page, emailFormField, EmailAlreadyInUseMessage)
	}

	page.Link = output.Link
	page.ExpiresAt = output.ExpiresAt
	return pages.Render(ctx, fiber.StatusOK, pageFiles, inviteUserTemplate, page)
}

func (this *InviteUserUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *InviteUserUiHandler) respondProblem(ctx fiber.Ctx, status int, page InviteUserPage, field string, message string) error {
	page.Field = field
	page.Message = message
	return pages.Render(ctx, status, pageFiles, inviteUserTemplate, page)
}
