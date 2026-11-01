package create_participant

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewCreateParticipantUseCase(participantRepository)
	apiHandler := NewCreateParticipantApiHandler(useCase)
	kernel.Server.Post("/api/participants", apiHandler.Handle)
}
