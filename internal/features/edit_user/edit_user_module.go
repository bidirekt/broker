package edit_user

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewEditUserUseCase(userRepository)
	uiHandler := NewEditUserUiHandler(useCase)
	kernel.Server.Get("/users/:id/edit", uiHandler.ShowForm)
	kernel.Server.Post("/users/:id/edit", uiHandler.EditUser)
}
