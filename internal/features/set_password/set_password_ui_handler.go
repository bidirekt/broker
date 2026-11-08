package set_password

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

//go:embed set_password.html set_password_link_invalid.html
var pageFiles embed.FS

type SetPasswordUiHandler struct {
	useCase       *SetPasswordUseCase
	sessionKeeper *sessions.Keeper
}

func NewSetPasswordUiHandler(useCase *SetPasswordUseCase, sessionKeeper *sessions.Keeper) *SetPasswordUiHandler {
	return &SetPasswordUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *SetPasswordUiHandler) ShowForm(ctx fiber.Ctx) error {
	token := ctx.Query(tokenField)
	link := this.useCase.FindLink(ctx.Context(), token)
	if link.Status != LinkFound {
		return this.respondLinkInvalid(ctx)
	}

	return this.render(ctx, fiber.StatusOK, token, link, "", "")
}

func (this *SetPasswordUiHandler) SetPassword(ctx fiber.Ctx) error {
	if !this.sessionKeeper.IsSameOrigin(ctx) {
		return this.respondForbidden(ctx)
	}

	token := ctx.FormValue(tokenField)
	output, err := this.useCase.Execute(ctx.Context(), SetPasswordInput{
		Token:    token,
		Password: ctx.FormValue(passwordFormField),
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case PasswordInvalid:
		return this.render(ctx, fiber.StatusBadRequest, token, output, passwordFormField, PasswordInvalidMessage)
	case LinkInvalid:
		return this.respondLinkInvalid(ctx)
	}

	return pages.Redirect(ctx, signInPath)
}

func (this *SetPasswordUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.ForbiddenMessage)
}

func (this *SetPasswordUiHandler) respondLinkInvalid(ctx fiber.Ctx) error {
	return pages.Render(ctx, fiber.StatusGone, pageFiles, linkInvalidTemplate, LinkInvalidPage{
		Message: LinkInvalidMessage,
	})
}

func (this *SetPasswordUiHandler) render(ctx fiber.Ctx, status int, token string, link FindLinkOutput, field, message string) error {
	return pages.Render(ctx, status, pageFiles, setPasswordTemplate, SetPasswordPage{
		Token:     token,
		Email:     link.Email,
		ExpiresAt: link.ExpiresAt,
		Field:     field,
		Message:   message,
	})
}
