package update_profile

import (
	"embed"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed update_profile.html
var pageFiles embed.FS

type UpdateProfileUiHandler struct {
	useCase       *UpdateProfileUseCase
	sessionKeeper *sessions.Keeper
}

func NewUpdateProfileUiHandler(useCase *UpdateProfileUseCase, sessionKeeper *sessions.Keeper) *UpdateProfileUiHandler {
	return &UpdateProfileUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *UpdateProfileUiHandler) ShowForm(ctx fiber.Ctx) error {
	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	return this.render(ctx, fiber.StatusOK, page, "", "")
}

func (this *UpdateProfileUiHandler) UpdateProfile(ctx fiber.Ctx) error {
	principal, _ := access.PrincipalFrom(ctx)
	session, _ := access.SessionFrom(ctx)
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
		return this.render(ctx, fiber.StatusBadRequest, firstPage, currentPasswordFormField, CurrentPasswordWrongMessage)
	case PasswordInvalid:
		return this.render(ctx, fiber.StatusBadRequest, firstPage, newPasswordFormField, PasswordInvalidMessage)
	}

	this.sessionKeeper.AddMessage(ctx, model.MessageSuccess, PasswordChangedMessage)
	return pages.Redirect(ctx, profilePath)
}

func (this *UpdateProfileUiHandler) RespondTooManyAttempts(ctx fiber.Ctx) error {
	return this.render(ctx, fiber.StatusTooManyRequests, firstPage, "", TooManyProfileAttemptsMessage)
}

func (this *UpdateProfileUiHandler) render(ctx fiber.Ctx, status int, page int64, field, message string) error {
	principal, _ := access.PrincipalFrom(ctx)
	apiKeys := this.useCase.ListAPIKeys(ctx.Context(), ListAPIKeysInput{UserID: principal.UserID, Page: page})
	pager, ok := pages.NewPager(ctx, pageParam, page, apiKeys.Total)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, status, pageFiles, updateProfileTemplate, UpdateProfilePage{
		Email:                 principal.Email,
		Permission:            principal.Permission,
		PermissionDescription: permissionDescriptions[principal.Permission],
		Field:                 field,
		Message:               message,
		APIKeys:               newAPIKeyRows(apiKeys.Keys, principal.Permission, time.Now()),
		Pager:                 pager,
	})
}
