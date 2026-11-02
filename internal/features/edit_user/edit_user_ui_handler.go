package edit_user

import (
	"embed"
	"strconv"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/gofiber/fiber/v3"
)

//go:embed edit_user.html
var pageFiles embed.FS

type EditUserUiHandler struct {
	useCase *EditUserUseCase
}

func NewEditUserUiHandler(useCase *EditUserUseCase) *EditUserUiHandler {
	return &EditUserUiHandler{useCase: useCase}
}

func (this *EditUserUiHandler) ShowForm(ctx fiber.Ctx) error {
	if err := access.RequireAdmin(ctx); err != nil {
		return err
	}

	userID, ok := this.userIDFrom(ctx)
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	user, found := this.useCase.FindUser(ctx.Context(), userID)
	if !found {
		return this.respondUserNotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, editUserTemplate, EditUserPage{
		UserID:     user.ID,
		Email:      user.Email,
		Permission: string(user.Permission),
	})
}

func (this *EditUserUiHandler) EditUser(ctx fiber.Ctx) error {
	if err := access.RequireAdmin(ctx); err != nil {
		return err
	}

	userID, ok := this.userIDFrom(ctx)
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	page := EditUserPage{
		UserID:     userID,
		Email:      ctx.FormValue(emailFormField),
		Permission: ctx.FormValue(permissionFormField),
	}

	output := this.useCase.Execute(ctx.Context(), EditUserInput{
		UserID:     userID,
		Email:      page.Email,
		Permission: page.Permission,
	})

	switch output.Status {
	case EmailInvalid:
		return this.respondWithMessage(ctx, fiber.StatusBadRequest, page, EmailInvalidMessage)
	case PermissionInvalid:
		return this.respondWithMessage(ctx, fiber.StatusBadRequest, page, PermissionInvalidMessage)
	case EmailAlreadyInUse:
		return this.respondWithMessage(ctx, fiber.StatusConflict, page, EmailAlreadyInUseMessage)
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	}

	return pages.Redirect(ctx, usersPath)
}

func (this *EditUserUiHandler) userIDFrom(ctx fiber.Ctx) (int64, bool) {
	rawUserID := ctx.Params(userIDParam)
	userID, err := strconv.ParseInt(rawUserID, 10, 64)

	return userID, err == nil && strconv.FormatInt(userID, 10) == rawUserID
}

func (this *EditUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return pages.Render(ctx, fiber.StatusNotFound, pageFiles, editUserTemplate, EditUserPage{
		Message: UserNotFoundMessage,
	})
}

func (this *EditUserUiHandler) respondWithMessage(ctx fiber.Ctx, status int, page EditUserPage, message string) error {
	page.Message = message
	return pages.Render(ctx, status, pageFiles, editUserTemplate, page)
}
