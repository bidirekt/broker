package generate_link

import (
	"embed"
	"strconv"

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

	rawUserID := ctx.Params(userIDParam)
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || strconv.FormatInt(userID, 10) != rawUserID {
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
		return this.respondWithMessage(ctx, fiber.StatusConflict, UserDeactivatedMessage)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, generateLinkTemplate, GenerateLinkPage{
		Email:   output.Email,
		Purpose: output.Purpose,
		Link:    output.Link,
	})
}

func (this *GenerateLinkUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *GenerateLinkUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}

func (this *GenerateLinkUiHandler) respondWithMessage(ctx fiber.Ctx, status int, message string) error {
	return pages.Render(ctx, status, pageFiles, generateLinkTemplate, GenerateLinkPage{Message: message})
}
