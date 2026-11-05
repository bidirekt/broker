package generate_link

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
)

//go:embed generate_link.html
var pageFiles embed.FS

type GenerateLinkUiHandler struct {
	useCase *GenerateLinkUseCase
}

func NewGenerateLinkUiHandler(useCase *GenerateLinkUseCase) *GenerateLinkUiHandler {
	return &GenerateLinkUiHandler{useCase: useCase}
}

func (this *GenerateLinkUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	userID, ok := pages.ParsePositiveInteger(ctx.Params(userIDParam))
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	output, err := this.useCase.Execute(ctx.Context(), GenerateLinkInput{UserID: userID})
	if err != nil {
		return err
	}

	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case UserDeactivated:
		return this.respondWithMessage(ctx, fiber.StatusConflict, output.Email, UserDeactivatedMessage)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, generateLinkTemplate, GenerateLinkPage{
		Email:            output.Email,
		Purpose:          output.Purpose,
		Link:             output.Link,
		ExpiresAt:        output.ExpiresAt,
		ReplacedOpenLink: output.ReplacedOpenLink,
	})
}

func (this *GenerateLinkUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *GenerateLinkUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}

func (this *GenerateLinkUiHandler) respondWithMessage(ctx fiber.Ctx, status int, email string, message string) error {
	return pages.Render(ctx, status, pageFiles, generateLinkTemplate, GenerateLinkPage{Email: email, Message: message})
}
