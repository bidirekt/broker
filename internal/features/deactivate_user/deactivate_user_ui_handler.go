package deactivate_user

import (
	"embed"
	"strconv"

	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

//go:embed deactivate_user.html
var pageFiles embed.FS

type DeactivateUserUiHandler struct {
	useCase *DeactivateUserUseCase
}

func NewDeactivateUserUiHandler(useCase *DeactivateUserUseCase) *DeactivateUserUiHandler {
	return &DeactivateUserUiHandler{useCase: useCase}
}

func (this *DeactivateUserUiHandler) Handle(ctx fiber.Ctx) error {
	if err := components.RequireAdmin(ctx); err != nil {
		return err
	}

	rawUserID := ctx.Params(userIDParam)
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || strconv.FormatInt(userID, 10) != rawUserID {
		return this.respondUserNotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), DeactivateUserInput{UserID: userID})
	if output.Status == UserNotFound {
		return this.respondUserNotFound(ctx)
	}

	return components.Redirect(ctx, usersPath)
}

func (this *DeactivateUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return components.Render(ctx, fiber.StatusNotFound, pageFiles, deactivateUserTemplate, DeactivateUserPage{Message: UserNotFoundMessage})
}
