package rename_participant

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	handler := NewRenameParticipantHandler(participantRepository)
	kernel.Server.Post("/api/participants/rename", handler.Handle)
}
