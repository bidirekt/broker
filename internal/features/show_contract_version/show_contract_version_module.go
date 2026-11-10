package show_contract_version

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	useCase := NewShowContractVersionUseCase(repository.NewContractVersionPageRepository(kernel.Pool))
	uiHandler := NewShowContractVersionUiHandler(useCase)
	kernel.Server.Get("/participants/:name/versions/:version", uiHandler.Handle)
}
