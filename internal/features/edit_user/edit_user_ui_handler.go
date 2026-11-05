package edit_user

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
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
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	userID, ok := pages.ParsePositiveInteger(ctx.Params(userIDParam))
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	returnPage, ok := pages.ReturnPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	target := this.useCase.FindUser(ctx.Context(), userID)
	switch target.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	}

	page := newEditUserPage(target.User, returnPage)
	page.Email = target.User.Email
	page.Permission = string(target.User.Permission)

	return pages.Render(ctx, fiber.StatusOK, pageFiles, editUserTemplate, page)
}

func (this *EditUserUiHandler) EditUser(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	userID, ok := pages.ParsePositiveInteger(ctx.Params(userIDParam))
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	returnPage, ok := pages.ReturnPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	email := ctx.FormValue(emailFormField)
	permission := ctx.FormValue(permissionFormField)
	output := this.useCase.Execute(ctx.Context(), EditUserInput{
		UserID:     userID,
		Email:      email,
		Permission: permission,
	})

	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	case UserEdited:
		return pages.Redirect(ctx, usersPathFor(returnPage))
	}

	page := newEditUserPage(output.User, returnPage)
	page.Email = email
	page.Permission = permission

	switch output.Status {
	case EmailInvalid:
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, emailFormField, EmailInvalidMessage)
	case PermissionInvalid:
		page.Permission = ""
		return this.respondProblem(ctx, fiber.StatusBadRequest, page, permissionFormField, PermissionInvalidMessage)
	}

	return this.respondProblem(ctx, fiber.StatusConflict, page, emailFormField, EmailAlreadyInUseMessage)
}

func newEditUserPage(stored *model.User, returnPage int64) EditUserPage {
	return EditUserPage{
		UserID:      stored.ID,
		StoredEmail: stored.Email,
		StateLine:   stateLines[stored.State()],
		ReturnPage:  returnPage,
		UsersPath:   usersPathFor(returnPage),
	}
}

func (this *EditUserUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *EditUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}

func (this *EditUserUiHandler) respondProblem(ctx fiber.Ctx, status int, page EditUserPage, field string, message string) error {
	page.Field = field
	page.Message = message
	return pages.Render(ctx, status, pageFiles, editUserTemplate, page)
}
