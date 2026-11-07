package show_participant_relations

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewShowParticipantRelationsUseCase(participantRepository)
	uiHandler := NewShowParticipantRelationsUiHandler(useCase)
	kernel.Server.Get("/participants/:name/relations", uiHandler.Handle)
}
