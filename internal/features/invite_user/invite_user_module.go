package invite_user

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewInviteUserUseCase(userRepository, kernel.Settings.PublicURL)
	uiHandler := NewInviteUserUiHandler(useCase)
	kernel.Server.Get("/users/invite", uiHandler.ShowForm)
	kernel.Server.Post("/users/invite", uiHandler.Invite)
}
