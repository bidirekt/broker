package list_environments

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)
	useCase := NewListEnvironmentsUseCase(environmentRepository)
	apiHandler := NewListEnvironmentsApiHandler(useCase)
	kernel.Server.Get("/api/environments", apiHandler.Handle)
}
