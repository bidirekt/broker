package list_participant_checks

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, reportReader *breakreport.Reader) {
	participantChecksRepository := repository.NewParticipantChecksRepository(kernel.Pool)
	useCase := NewListParticipantChecksUseCase(participantChecksRepository, reportReader)
	uiHandler := NewListParticipantChecksUiHandler(useCase)
	kernel.Server.Get("/participants/:name/checks", uiHandler.Handle)
}
