package revoke_api_key

import (
	"fmt"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

type RevokeAPIKeyUiHandler struct {
	useCase       *RevokeAPIKeyUseCase
	sessionKeeper *sessions.Keeper
}

func NewRevokeAPIKeyUiHandler(useCase *RevokeAPIKeyUseCase, sessionKeeper *sessions.Keeper) *RevokeAPIKeyUiHandler {
	return &RevokeAPIKeyUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *RevokeAPIKeyUiHandler) Handle(ctx fiber.Ctx) error {
	apiKeyID, ok := pages.ParsePositiveInteger(ctx.Params(apiKeyIDParam))
	if !ok {
		return this.respondAPIKeyNotFound(ctx)
	}

	principal, _ := access.PrincipalFrom(ctx)
	output := this.useCase.Execute(ctx.Context(), RevokeAPIKeyInput{APIKeyID: apiKeyID, UserID: principal.UserID})
	if output.Status == APIKeyNotFound {
		return this.respondAPIKeyNotFound(ctx)
	}

	this.sessionKeeper.AddMessage(ctx, model.MessageSuccess, fmt.Sprintf(APIKeyRevokedMessageFormat, output.Name))
	return pages.Redirect(ctx, profilePath)
}

func (this *RevokeAPIKeyUiHandler) respondAPIKeyNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, APIKeyNotFoundMessage)
}
