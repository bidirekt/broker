package create_environment

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)
	useCase := NewCreateEnvironmentUseCase(environmentRepository)
	apiHandler := NewCreateEnvironmentApiHandler(useCase)
	kernel.Server.Post("/api/environments", apiHandler.Handle)
}
