package sign_in

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewSignInUseCase(userRepository, kernel.Settings.AdminUsername, kernel.Settings.AdminPassword)
	uiHandler := NewSignInUiHandler(useCase)
	kernel.Server.Get("/sign-in", uiHandler.ShowForm)
	kernel.Server.Post("/sign-in", uiHandler.SignIn)
	kernel.Server.Get(homePath, uiHandler.RedirectHome)
}
