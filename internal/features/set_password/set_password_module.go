package set_password

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	oneTimeLinkRepository := repository.NewOneTimeLinkRepository(kernel.Pool)
	useCase := NewSetPasswordUseCase(userRepository, oneTimeLinkRepository)
	uiHandler := NewSetPasswordUiHandler(useCase)
	kernel.Server.Get("/set-password", uiHandler.ShowForm)
	kernel.Server.Post("/set-password", uiHandler.SetPassword)
}
