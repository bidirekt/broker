package rename_environment

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)
	useCase := NewRenameEnvironmentUseCase(environmentRepository)
	apiHandler := NewRenameEnvironmentApiHandler(useCase)
	uiHandler := NewRenameEnvironmentUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post("/api/environments/rename", apiHandler.Handle)
	kernel.Server.Get("/environments/:id/rename", uiHandler.ShowForm)
	kernel.Server.Post("/environments/:id/rename", uiHandler.RenameEnvironment)
}
