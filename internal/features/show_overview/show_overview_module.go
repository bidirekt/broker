package show_overview

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, reportReader *breakreport.Reader) {
	useCase := NewShowOverviewUseCase(repository.NewOverviewRepository(kernel.Pool), reportReader)
	uiHandler := NewShowOverviewUiHandler(useCase)
	kernel.Server.Get("/", uiHandler.Handle)
}
