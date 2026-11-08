package sign_in

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

//go:embed sign_in.html
var pageFiles embed.FS

type SignInUiHandler struct {
	useCase       *SignInUseCase
	sessionKeeper *sessions.Keeper
}

func NewSignInUiHandler(useCase *SignInUseCase, sessionKeeper *sessions.Keeper) *SignInUiHandler {
	return &SignInUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *SignInUiHandler) ShowForm(ctx fiber.Ctx) error {
	return pages.Render(ctx, fiber.StatusOK, pageFiles, signInTemplate, SignInPage{
		Next: safeNextPathFrom(ctx.Query(nextField)),
	})
}

func (this *SignInUiHandler) RequireSameOrigin(ctx fiber.Ctx) error {
	if !this.sessionKeeper.IsSameOrigin(ctx) {
		return this.respondForbidden(ctx)
	}

	return ctx.Next()
}

func (this *SignInUiHandler) SignIn(ctx fiber.Ctx) error {
	email := this.Email(ctx)
	output := this.useCase.Execute(ctx.Context(), SignInInput{
		Email:    email,
		Password: ctx.FormValue(passwordFormField),
	})

	if output.Status == InvalidCredentials {
		return this.respondInvalidCredentials(ctx, email)
	}

	started, err := this.sessionKeeper.Start(ctx, output.UserID, output.VerifiedPasswordHash)
	if err != nil {
		return err
	}

	if !started {
		return this.respondInvalidCredentials(ctx, email)
	}

	return pages.Redirect(ctx, this.next(ctx))
}

func (this *SignInUiHandler) Email(ctx fiber.Ctx) string {
	return ctx.FormValue(emailFormField)
}

func (this *SignInUiHandler) next(ctx fiber.Ctx) string {
	return safeNextPathFrom(ctx.FormValue(nextField))
}

func (this *SignInUiHandler) RespondTooManyAttempts(ctx fiber.Ctx) error {
	return pages.Render(ctx, fiber.StatusTooManyRequests, pageFiles, signInTemplate, SignInPage{
		Email:   this.Email(ctx),
		Next:    this.next(ctx),
		Message: TooManySignInAttemptsMessage,
	})
}

func (this *SignInUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.ForbiddenMessage)
}

func (this *SignInUiHandler) respondInvalidCredentials(ctx fiber.Ctx, email string) error {
	return pages.Render(ctx, fiber.StatusUnauthorized, pageFiles, signInTemplate, SignInPage{
		Email:   email,
		Next:    this.next(ctx),
		Message: PageInvalidCredentialsMessage,
	})
}
