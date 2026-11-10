package show_compatibility

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components, reportReader *breakreport.Reader) {
	useCase := NewShowCompatibilityUseCase(
		repository.NewParticipantRepository(kernel.Pool),
		repository.NewCompatibilityPageRepository(kernel.Pool),
		reportReader,
	)
	uiHandler := NewShowCompatibilityUiHandler(useCase)
	kernel.Server.Get("/compatibility", uiHandler.Handle)
}
