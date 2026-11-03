package sign_out

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	sessionRepository := repository.NewSessionRepository(kernel.Pool)
	useCase := NewSignOutUseCase(sessionRepository)
	uiHandler := NewSignOutUiHandler(useCase, kernel.SessionKeeper)
	kernel.Server.Post(signOutPath, uiHandler.SignOut)
}
