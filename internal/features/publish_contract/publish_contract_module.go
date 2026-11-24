package publish_contract

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, interpreter *contractfiles.Interpreter, tables []repository.ProtocolTable) {
	contractRepository := repository.NewContractRepository(kernel.Pool, tables)
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	useCase := NewPublishContractUseCase(interpreter, contractRepository, participantRepository)
	apiHandler := NewPublishContractApiHandler(useCase)
	kernel.Server.Post("/api/contracts", apiHandler.Handle)
}
