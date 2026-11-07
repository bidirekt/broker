package list_participant_pages

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantListingRepository := repository.NewParticipantListingRepository(kernel.Pool)
	useCase := NewListParticipantPagesUseCase(participantListingRepository)
	uiHandler := NewListParticipantPagesUiHandler(useCase)
	kernel.Server.Get("/participants", uiHandler.Handle)
}
