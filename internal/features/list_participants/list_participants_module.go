package list_participants

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewListParticipantsUseCase(participantRepository)
	apiHandler := NewListParticipantsApiHandler(useCase)
	kernel.Server.Get("/api/participants", apiHandler.Handle)
}
