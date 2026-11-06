package reactivate_user

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewReactivateUserUseCase(userRepository)
	uiHandler := NewReactivateUserUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post("/users/:id/reactivate", uiHandler.Handle)
}
