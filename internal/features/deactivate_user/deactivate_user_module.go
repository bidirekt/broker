package deactivate_user

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewDeactivateUserUseCase(userRepository)
	uiHandler := NewDeactivateUserUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post("/users/:id/deactivate", uiHandler.Handle)
}
