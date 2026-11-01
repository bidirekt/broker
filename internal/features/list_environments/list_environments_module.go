package list_environments

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)
	handler := NewListEnvironmentsHandler(environmentRepository)
	kernel.Server.Get("/api/environments", handler.Handle)
}
