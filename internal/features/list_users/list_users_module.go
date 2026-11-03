package list_users

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewListUsersUseCase(userRepository)
	uiHandler := NewListUsersUiHandler(useCase)
	kernel.Server.Get("/users", uiHandler.Handle)
}
