package sign_in

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/sessions"
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
	return pages.Render(ctx, fiber.StatusOK, pageFiles, signInTemplate, SignInPage{})
}

func (this *SignInUiHandler) RequireSameOrigin(ctx fiber.Ctx) error {
	if err := sessions.RequireSameOrigin(ctx); err != nil {
		return err
	}

	return ctx.Next()
}

func (this *SignInUiHandler) SignIn(ctx fiber.Ctx) error {
	identifier := this.Identifier(ctx)
	output := this.useCase.Execute(ctx.Context(), SignInInput{
		Identifier: identifier,
		Password:   ctx.FormValue(passwordFormField),
	})

	if output.Status == InvalidCredentials {
		return this.respondInvalidCredentials(ctx, identifier)
	}

	started, err := sessions.Start(ctx, output.UserID, output.VerifiedPasswordHash)
	if err != nil {
		return err
	}

	if !started {
		return this.respondInvalidCredentials(ctx, identifier)
	}

	return pages.Redirect(ctx, homePath)
}

func (this *SignInUiHandler) Identifier(ctx fiber.Ctx) string {
	return ctx.FormValue(identifierFormField)
}

func (this *SignInUiHandler) respondInvalidCredentials(ctx fiber.Ctx, identifier string) error {
	return pages.Render(ctx, fiber.StatusUnauthorized, pageFiles, signInTemplate, SignInPage{
		Identifier: identifier,
		Message:    PageInvalidCredentialsMessage,
	})
}

func (this *SignInUiHandler) RedirectHome(ctx fiber.Ctx) error {
	if principal, _ := access.PrincipalFrom(ctx); principal.IsAdmin {
		return pages.Redirect(ctx, adminHomePath)
	}

	return pages.Redirect(ctx, userHomePath)
}
