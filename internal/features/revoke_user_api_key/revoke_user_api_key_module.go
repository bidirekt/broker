package revoke_user_api_key

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	apiKeyRepository := repository.NewAPIKeyRepository(kernel.Pool)
	useCase := NewRevokeUserAPIKeyUseCase(userRepository, apiKeyRepository)
	uiHandler := NewRevokeUserAPIKeyUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post("/users/:id/api-keys/:keyId/revoke", uiHandler.Handle)
}
