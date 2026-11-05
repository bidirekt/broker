package reactivate_user

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

type ReactivateUserUiHandler struct {
	useCase       *ReactivateUserUseCase
	sessionKeeper *sessions.Keeper
}

func NewReactivateUserUiHandler(useCase *ReactivateUserUseCase, sessionKeeper *sessions.Keeper) *ReactivateUserUiHandler {
	return &ReactivateUserUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *ReactivateUserUiHandler) Handle(ctx fiber.Ctx) error {
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

	output := this.useCase.Execute(ctx.Context(), ReactivateUserInput{UserID: userID})
	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	case UserReactivated:
		this.sessionKeeper.SetMessage(ctx, UserReactivatedMessage+output.User.Email)
	}

	return pages.Redirect(ctx, usersPathFor(returnPage))
}

func (this *ReactivateUserUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *ReactivateUserUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}
