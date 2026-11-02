package generate_link

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	linkRepository := repository.NewOneTimeLinkRepository(kernel.Pool)
	useCase := NewGenerateLinkUseCase(userRepository, linkRepository, kernel.Settings.PublicURL)
	uiHandler := NewGenerateLinkUiHandler(useCase)
	kernel.Server.Post("/users/:id/link", uiHandler.Handle)
}
