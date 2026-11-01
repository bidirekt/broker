package create_environment

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	repo := repository.NewEnvironmentRepository(kernel.Pool)
	handler := NewCreateEnvironmentHandler(repo)
	kernel.Server.Post("/api/environments", handler.Handle)
}
