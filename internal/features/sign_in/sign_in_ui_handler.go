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

	identifier := this.Identifier(ctx)
	output := this.useCase.Execute(ctx.Context(), SignInInput{
		Identifier: identifier,
		Password:   ctx.FormValue(passwordFormField),
	})

	if output.Status == InvalidCredentials {
		return this.respondInvalidCredentials(ctx, identifier)
	}

	started, err := components.StartSession(ctx, output.UserID, output.VerifiedPasswordHash)
	if err != nil {
		return err
	}

	if !started {
		return this.respondInvalidCredentials(ctx, identifier)
	}

	return components.Redirect(ctx, homePath)
}

func (this *SignInUiHandler) Identifier(ctx fiber.Ctx) string {
	return ctx.FormValue(identifierFormField)
}

func (this *SignInUiHandler) respondInvalidCredentials(ctx fiber.Ctx, identifier string) error {
	return components.Render(ctx, fiber.StatusUnauthorized, pageFiles, signInTemplate, SignInPage{
		Identifier: identifier,
		Message:    PageInvalidCredentialsMessage,
	})
}

func (this *SignInUiHandler) RedirectHome(ctx fiber.Ctx) error {
	if principal, _ := components.PrincipalFrom(ctx); principal.IsAdmin {
		return components.Redirect(ctx, adminHomePath)
	}

	return components.Redirect(ctx, userHomePath)
}
