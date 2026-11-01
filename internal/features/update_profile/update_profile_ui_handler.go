package update_profile

import (
	"embed"

	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

//go:embed update_profile.html
var pageFiles embed.FS

type UpdateProfileUiHandler struct {
	useCase *UpdateProfileUseCase
}

func NewUpdateProfileUiHandler(useCase *UpdateProfileUseCase) *UpdateProfileUiHandler {
	return &UpdateProfileUiHandler{useCase: useCase}
}

func (this *UpdateProfileUiHandler) ShowForm(ctx fiber.Ctx) error {
	if err := components.RequireUser(ctx); err != nil {
		return err
	}

	return components.Render(ctx, fiber.StatusOK, pageFiles, updateProfileTemplate, UpdateProfilePage{})
}

func (this *UpdateProfileUiHandler) UpdateProfile(ctx fiber.Ctx) error {
	if err := components.RequireUser(ctx); err != nil {
		return err
	}

	principal, _ := components.PrincipalFrom(ctx)
	session, _ := components.SessionFrom(ctx)
	output, err := this.useCase.Execute(ctx.Context(), UpdateProfileInput{
		UserID:          principal.UserID,
		SessionIDHash:   session.IDHash,
		CurrentPassword: ctx.FormValue(currentPasswordFormField),
		NewPassword:     ctx.FormValue(newPasswordFormField),
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case CurrentPasswordWrong:
		return this.respondProblem(ctx, CurrentPasswordWrongMessage)
	case PasswordInvalid:
		return this.respondProblem(ctx, PasswordInvalidMessage)
	}

	return components.Render(ctx, fiber.StatusOK, pageFiles, updateProfileTemplate, UpdateProfilePage{
		Confirmation: PasswordChangedMessage,
	})
}

func (this *UpdateProfileUiHandler) respondProblem(ctx fiber.Ctx, message string) error {
	return components.Render(ctx, fiber.StatusBadRequest, pageFiles, updateProfileTemplate, UpdateProfilePage{
		Problem: message,
	})
}
