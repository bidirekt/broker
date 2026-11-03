package update_profile

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewUpdateProfileUseCase(userRepository)
	uiHandler := NewUpdateProfileUiHandler(useCase)
	kernel.Server.Get(profilePath, uiHandler.ShowForm)
	kernel.Server.Post(profilePath, newUpdateProfileLimiter(), uiHandler.UpdateProfile)
}
