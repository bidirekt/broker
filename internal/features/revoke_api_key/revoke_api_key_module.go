package revoke_api_key

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	apiKeyRepository := repository.NewAPIKeyRepository(kernel.Pool)
	useCase := NewRevokeAPIKeyUseCase(apiKeyRepository)
	uiHandler := NewRevokeAPIKeyUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post("/profile/api-keys/:id/revoke", uiHandler.Handle)
}
