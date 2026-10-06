package list_environments

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(components *components.Components) {
	environmentRepository := repository.NewEnvironmentRepository(components.Pool)
	handler := NewListEnvironmentsHandler(environmentRepository)
	components.Server.Get("/api/environments", handler.Handle)
}
