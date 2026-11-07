package show_compatibility

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	useCase := NewShowCompatibilityUseCase(
		repository.NewParticipantRepository(kernel.Pool),
		repository.NewCompatibilityPageRepository(kernel.Pool),
	)
	uiHandler := NewShowCompatibilityUiHandler(useCase)
	kernel.Server.Get("/compatibility", uiHandler.Handle)
}
