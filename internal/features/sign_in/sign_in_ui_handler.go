package sign_in

import (
	"embed"

	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

//go:embed sign_in.html
var pageFiles embed.FS

type SignInUiHandler struct {
	useCase *SignInUseCase
}

func NewSignInUiHandler(useCase *SignInUseCase) *SignInUiHandler {
	return &SignInUiHandler{useCase: useCase}
}

func (this *SignInUiHandler) ShowForm(ctx fiber.Ctx) error {
	return components.Render(ctx, fiber.StatusOK, pageFiles, signInTemplate, SignInPage{})
}

func (this *SignInUiHandler) SignIn(ctx fiber.Ctx) error {
	if err := components.RequireSameOrigin(ctx); err != nil {
		return err
	}

	identifier := ctx.FormValue(identifierFormField)
	output := this.useCase.Execute(ctx.Context(), SignInInput{
		Identifier: identifier,
		Password:   ctx.FormValue(passwordFormField),
	})

	if output.Status == InvalidCredentials {
		return components.Render(ctx, fiber.StatusUnauthorized, pageFiles, signInTemplate, SignInPage{
			Identifier: identifier,
			Message:    InvalidCredentialsMessage,
		})
	}

	if err := components.StartSession(ctx, output.UserID); err != nil {
		return err
	}

	return components.Redirect(ctx, homePath)
}

func (this *SignInUiHandler) RedirectHome(ctx fiber.Ctx) error {
	if principal, _ := components.PrincipalFrom(ctx); principal.IsAdmin {
		return components.Redirect(ctx, adminHomePath)
	}

	return components.Redirect(ctx, userHomePath)
}
