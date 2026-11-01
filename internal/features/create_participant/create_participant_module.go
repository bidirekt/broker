package create_participant

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	repo := repository.NewParticipantRepository(kernel.Pool)
	handler := NewCreateParticipantHandler(repo)
	kernel.Server.Post("/api/participants", handler.Handle)
}
