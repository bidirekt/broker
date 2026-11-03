package setup_admin

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

//go:embed setup.html
var pageFiles embed.FS

type SetupAdminUiHandler struct {
	useCase *SetupAdminUseCase
}

func NewSetupAdminUiHandler(useCase *SetupAdminUseCase) *SetupAdminUiHandler {
	return &SetupAdminUiHandler{useCase: useCase}
}

func (this *SetupAdminUiHandler) ShowForm(ctx fiber.Ctx) error {
	if this.useCase.IsSetUp(ctx.Context()) {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, setupTemplate, SetupPage{})
}

func (this *SetupAdminUiHandler) SetUp(ctx fiber.Ctx) error {
	if !sessions.IsSameOrigin(ctx) {
		return this.respondForbidden(ctx)
	}

	page := SetupPage{Email: ctx.FormValue(emailFormField)}
	output, err := this.useCase.Execute(ctx.Context(), SetupAdminInput{
		Email:    page.Email,
		Password: ctx.FormValue(passwordFormField),
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case AlreadySetUp:
		return respond.NotFound(ctx)
	case EmailInvalid:
		return this.respondProblem(ctx, page, EmailInvalidMessage)
	case PasswordInvalid:
		return this.respondProblem(ctx, page, PasswordInvalidMessage)
	case SetUpConcurrently:
		return pages.Redirect(ctx, signInPath)
	}

	started, err := sessions.Start(ctx, output.UserID, output.PasswordHash)
	if err != nil {
		return err
	}

	if !started {
		return pages.Redirect(ctx, signInPath)
	}

	return pages.Redirect(ctx, usersPath)
}

func (this *SetupAdminUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.ForbiddenMessage)
}

func (this *SetupAdminUiHandler) respondProblem(ctx fiber.Ctx, page SetupPage, message string) error {
	page.Message = message
	return pages.Render(ctx, fiber.StatusBadRequest, pageFiles, setupTemplate, page)
}
