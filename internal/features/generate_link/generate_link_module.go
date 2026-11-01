package generate_link

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	linkRepository := repository.NewOneTimeLinkRepository(kernel.Pool)
	useCase := NewGenerateLinkUseCase(linkRepository, kernel.Settings.PublicURL)
	uiHandler := NewGenerateLinkUiHandler(useCase)
	kernel.Server.Post("/users/:id/link", uiHandler.Handle)
}
