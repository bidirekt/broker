package setup_admin

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewSetupAdminUseCase(userRepository)
	uiHandler := NewSetupAdminUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Get("/setup", uiHandler.ShowForm)
	kernel.Server.Post("/setup", uiHandler.SetUp)
}
