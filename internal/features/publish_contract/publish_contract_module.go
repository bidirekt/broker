package publish_contract

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	contractRepository := repository.NewContractRepository(kernel.Pool)
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewPublishContractUseCase(contractRepository, participantRepository)
	apiHandler := NewPublishContractApiHandler(useCase)
	kernel.Server.Post("/api/contracts", apiHandler.Handle)
}
