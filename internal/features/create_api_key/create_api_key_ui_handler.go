package create_api_key

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed create_api_key.html
var pageFiles embed.FS

type CreateAPIKeyUiHandler struct {
	useCase *CreateAPIKeyUseCase
}

func NewCreateAPIKeyUiHandler(useCase *CreateAPIKeyUseCase) *CreateAPIKeyUiHandler {
	return &CreateAPIKeyUiHandler{useCase: useCase}
}

func (this *CreateAPIKeyUiHandler) ShowForm(ctx fiber.Ctx) error {
	principal, _ := access.PrincipalFrom(ctx)
	permission := model.PermissionRead
	if principal.CanWrite() {
		permission = model.PermissionWrite
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, createAPIKeyTemplate, CreateAPIKeyPage{
		Permission: string(permission),
		Expiration: defaultExpiration,
	})
}

func (this *CreateAPIKeyUiHandler) CreateAPIKey(ctx fiber.Ctx) error {
	principal, _ := access.PrincipalFrom(ctx)
	page := CreateAPIKeyPage{
		Name:       ctx.FormValue(nameFormField),
		Permission: ctx.FormValue(permissionFormField),
		Expiration: ctx.FormValue(expirationFormField),
	}

	output, err := this.useCase.Execute(ctx.Context(), CreateAPIKeyInput{
		UserID:          principal.UserID,
		OwnerPermission: principal.Permission,
		Name:            page.Name,
		Permission:      page.Permission,
		Expiration:      page.Expiration,
	})
	if err != nil {
		return err
	}

	switch output.Status {
	case NameMissing:
		return this.respondProblem(ctx, page, nameFormField, NameMissingMessage)
	case NameTooLong:
		return this.respondProblem(ctx, page, nameFormField, NameTooLongMessage)
	case PermissionInvalid:
		page.Permission = ""
		return this.respondProblem(ctx, page, permissionFormField, PermissionInvalidMessage)
	case PermissionAboveOwn:
		page.Permission = ""
		return this.respondProblem(ctx, page, permissionFormField, PermissionAboveOwnMessage)
	case ExpirationInvalid:
		page.Expiration = ""
		return this.respondProblem(ctx, page, expirationFormField, ExpirationInvalidMessage)
	case OwnerRefused:
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	page.Key = output.Key
	return pages.Render(ctx, fiber.StatusOK, pageFiles, createAPIKeyTemplate, page)
}

func (this *CreateAPIKeyUiHandler) respondProblem(ctx fiber.Ctx, page CreateAPIKeyPage, field string, message string) error {
	page.Field = field
	page.Message = message
	return pages.Render(ctx, fiber.StatusBadRequest, pageFiles, createAPIKeyTemplate, page)
}
