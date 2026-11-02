package set_password

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

//go:embed set_password.html set_password_link_invalid.html
var pageFiles embed.FS

type SetPasswordUiHandler struct {
	useCase *SetPasswordUseCase
}

func NewSetPasswordUiHandler(useCase *SetPasswordUseCase) *SetPasswordUiHandler {
	return &SetPasswordUiHandler{useCase: useCase}
}

func (this *SetPasswordUiHandler) ShowForm(ctx fiber.Ctx) error {
	token := ctx.Query(tokenField)
	if !this.useCase.IsLinkUsable(ctx.Context(), token) {
		return this.respondLinkInvalid(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, setPasswordTemplate, SetPasswordPage{Token: token})
}

func (this *SetPasswordUiHandler) SetPassword(ctx fiber.Ctx) error {
	if err := sessions.RequireSameOrigin(ctx); err != nil {
		return err
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
		return pages.Render(ctx, fiber.StatusBadRequest, pageFiles, setPasswordTemplate, SetPasswordPage{
			Token:   token,
			Message: PasswordInvalidMessage,
		})
	case LinkInvalid:
		return this.respondLinkInvalid(ctx)
	}

	return pages.Redirect(ctx, signInPath)
}

func (this *SetPasswordUiHandler) respondLinkInvalid(ctx fiber.Ctx) error {
	return pages.Render(ctx, fiber.StatusGone, pageFiles, linkInvalidTemplate, LinkInvalidPage{
		Message: LinkInvalidMessage,
	})
}
