package deactivate_user

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

type DeactivateUserUiHandler struct {
	useCase       *DeactivateUserUseCase
	sessionKeeper *sessions.Keeper
}

func NewDeactivateUserUiHandler(useCase *DeactivateUserUseCase, sessionKeeper *sessions.Keeper) *DeactivateUserUiHandler {
	return &DeactivateUserUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *DeactivateUserUiHandler) Handle(ctx fiber.Ctx) error {
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

	output := this.useCase.Execute(ctx.Context(), DeactivateUserInput{UserID: userID})
	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	case UserDeactivated:
		this.sessionKeeper.SetMessage(ctx, UserDeactivatedMessage+output.User.Email)
	}

	return pages.Redirect(ctx, usersPathFor(returnPage))
}

func (this *DeactivateUserUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *DeactivateUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}
