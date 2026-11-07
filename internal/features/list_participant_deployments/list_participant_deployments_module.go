package list_participant_deployments

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantDeploymentsRepository := repository.NewParticipantDeploymentsRepository(kernel.Pool)
	useCase := NewListParticipantDeploymentsUseCase(participantDeploymentsRepository)
	uiHandler := NewListParticipantDeploymentsUiHandler(useCase)
	kernel.Server.Get("/participants/:name/deployments", uiHandler.Handle)
}
