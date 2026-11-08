package revoke_user_api_key

import (
	"fmt"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type RevokeUserAPIKeyUiHandler struct {
	useCase       *RevokeUserAPIKeyUseCase
	sessionKeeper *sessions.Keeper
}

func NewRevokeUserAPIKeyUiHandler(useCase *RevokeUserAPIKeyUseCase, sessionKeeper *sessions.Keeper) *RevokeUserAPIKeyUiHandler {
	return &RevokeUserAPIKeyUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *RevokeUserAPIKeyUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	userID, ok := pages.ParsePositiveInteger(ctx.Params(userIDParam))
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	apiKeyID, ok := pages.ParsePositiveInteger(ctx.Params(apiKeyIDParam))
	if !ok {
		return this.respondAPIKeyNotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), RevokeUserAPIKeyInput{UserID: userID, APIKeyID: apiKeyID})
	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	case APIKeyNotFound:
		return this.respondAPIKeyNotFound(ctx)
	}

	this.sessionKeeper.AddMessage(ctx, model.MessageSuccess, fmt.Sprintf(APIKeyRevokedMessageFormat, output.Name))
	return pages.Redirect(ctx, userAPIKeysPath(userID))
}

func (this *RevokeUserAPIKeyUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *RevokeUserAPIKeyUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}

func (this *RevokeUserAPIKeyUiHandler) respondAPIKeyNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, APIKeyNotFoundMessage)
}
