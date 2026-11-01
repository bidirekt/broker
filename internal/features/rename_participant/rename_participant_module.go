package rename_participant

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewRenameParticipantUseCase(participantRepository)
	apiHandler := NewRenameParticipantApiHandler(useCase)
	kernel.Server.Post("/api/participants/rename", apiHandler.Handle)
}
