package show_overview

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	useCase := NewShowOverviewUseCase(repository.NewOverviewRepository(kernel.Pool))
	uiHandler := NewShowOverviewUiHandler(useCase)
	kernel.Server.Get("/", uiHandler.Handle)
}
