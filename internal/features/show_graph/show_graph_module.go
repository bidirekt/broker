package show_graph

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
	graphqlrepository "github.com/bidirekt/broker/internal/repository/protocols/graphql"
)

func Register(kernel *components.Components, contracts *repository.ContractRepository) {
	useCase := NewShowGraphUseCase(
		graphqlrepository.NewGraphLoader(kernel.Pool),
		repository.NewEnvironmentRepository(kernel.Pool),
		contracts,
	)
	uiHandler := NewShowGraphUiHandler(useCase)
	kernel.Server.Get("/graphs/:name", uiHandler.Handle)
}
