package deactivate_user

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
)

type DeactivateUserUiHandler struct {
	useCase *DeactivateUserUseCase
}

func NewDeactivateUserUiHandler(useCase *DeactivateUserUseCase) *DeactivateUserUiHandler {
	return &DeactivateUserUiHandler{useCase: useCase}
}

func (this *DeactivateUserUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	userID, ok := pages.ParsePositiveInteger(ctx.Params(userIDParam))
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), DeactivateUserInput{UserID: userID})
	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	}

	return pages.Redirect(ctx, usersPath)
}

func (this *DeactivateUserUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *DeactivateUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}
