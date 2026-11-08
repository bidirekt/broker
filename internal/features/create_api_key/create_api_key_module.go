package create_api_key

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	apiKeyRepository := repository.NewAPIKeyRepository(kernel.Pool)
	useCase := NewCreateAPIKeyUseCase(apiKeyRepository)
	uiHandler := NewCreateAPIKeyUiHandler(useCase)
	kernel.Server.Get(newAPIKeyPath, uiHandler.ShowForm)
	kernel.Server.Post(newAPIKeyPath, uiHandler.CreateAPIKey)
}
