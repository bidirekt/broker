package publish_contract

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	contractRepository := repository.NewContractRepository(kernel.Pool)
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	handler := NewPublishContractHandler(contractRepository, participantRepository)
	kernel.Server.Post("/api/contracts", handler.Handle)
}
