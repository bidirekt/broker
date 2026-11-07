package create_participant

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewCreateParticipantUseCase(participantRepository)
	apiHandler := NewCreateParticipantApiHandler(useCase)
	uiHandler := NewCreateParticipantUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post("/api/participants", apiHandler.Handle)
	kernel.Server.Get("/participants/new-participant", uiHandler.ShowForm)
	kernel.Server.Post("/participants/new-participant", uiHandler.Create)
}
