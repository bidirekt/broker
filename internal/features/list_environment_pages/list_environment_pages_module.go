package list_environment_pages

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	environmentListingRepository := repository.NewEnvironmentListingRepository(kernel.Pool)
	useCase := NewListEnvironmentPagesUseCase(environmentListingRepository)
	uiHandler := NewListEnvironmentPagesUiHandler(useCase)
	kernel.Server.Get("/environments", uiHandler.Handle)
}
