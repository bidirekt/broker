package list_user_api_keys

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	apiKeyRepository := repository.NewAPIKeyRepository(kernel.Pool)
	useCase := NewListUserAPIKeysUseCase(userRepository, apiKeyRepository)
	uiHandler := NewListUserAPIKeysUiHandler(useCase)
	kernel.Server.Get("/users/:id/api-keys", uiHandler.Handle)
}
