package list_participants

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(components *components.Components) {
	participantRepository := repository.NewParticipantRepository(components.Pool)
	handler := NewListParticipantsHandler(participantRepository)
	components.Server.Get("/api/participants", handler.Handle)
}
