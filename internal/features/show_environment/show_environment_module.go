package show_environment

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	useCase := NewShowEnvironmentUseCase(
		repository.NewEnvironmentPageRepository(kernel.Pool),
		repository.NewDeploymentRepository(kernel.Pool),
	)
	uiHandler := NewShowEnvironmentUiHandler(useCase)
	kernel.Server.Get("/environments/:id", uiHandler.Handle)
}
