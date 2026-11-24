package show_participant_versions

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, links []repository.Links) {
	participantVersionsRepository := repository.NewParticipantVersionsRepository(kernel.Pool)
	useCase := NewShowParticipantVersionsUseCase(participantVersionsRepository, repository.NewProtocolLinks(kernel.Pool, links))
	uiHandler := NewShowParticipantVersionsUiHandler(useCase)
	kernel.Server.Get("/participants/:name", uiHandler.Handle)
}
