package show_environment

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, reportReader *breakreport.Reader) {
	useCase := NewShowEnvironmentUseCase(
		repository.NewEnvironmentPageRepository(kernel.Pool),
		repository.NewDeploymentRepository(kernel.Pool),
		reportReader,
	)
	uiHandler := NewShowEnvironmentUiHandler(useCase)
	kernel.Server.Get("/environments/:id", uiHandler.Handle)
}
