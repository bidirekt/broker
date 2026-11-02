package reactivate_user

import (
	"embed"
	"strconv"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
)

//go:embed reactivate_user.html
var pageFiles embed.FS

type ReactivateUserUiHandler struct {
	useCase *ReactivateUserUseCase
}

func NewReactivateUserUiHandler(useCase *ReactivateUserUseCase) *ReactivateUserUiHandler {
	return &ReactivateUserUiHandler{useCase: useCase}
}

func (this *ReactivateUserUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	rawUserID := ctx.Params(userIDParam)
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || strconv.FormatInt(userID, 10) != rawUserID {
		return this.respondUserNotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ReactivateUserInput{UserID: userID})
	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	}

	return pages.Redirect(ctx, usersPath)
}

func (this *ReactivateUserUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *ReactivateUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return pages.Render(ctx, fiber.StatusNotFound, pageFiles, reactivateUserTemplate, ReactivateUserPage{Message: UserNotFoundMessage})
}
