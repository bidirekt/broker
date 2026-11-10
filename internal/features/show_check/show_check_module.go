package show_check

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, reportReader *breakreport.Reader) {
	compatibilityRepository := repository.NewCompatibilityRepository(kernel.Pool)
	useCase := NewShowCheckUseCase(compatibilityRepository, reportReader)
	uiHandler := NewShowCheckUiHandler(useCase)
	kernel.Server.Get("/checks/:id", uiHandler.Handle)
}
