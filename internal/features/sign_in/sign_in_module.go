package sign_in

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	userRepository := repository.NewUserRepository(kernel.Pool)
	useCase := NewSignInUseCase(userRepository)
	uiHandler := NewSignInUiHandler(useCase, kernel.SessionKeeper)
	apiHandler := NewSignInApiHandler(useCase, kernel.Settings)
	limiter := newSignInLimiter()
	kernel.Server.Get("/sign-in", uiHandler.ShowForm)
	kernel.Server.Post("/sign-in", uiHandler.RequireSameOrigin, limiter.PerIP, limiter.PerAccount(uiHandler.Email), uiHandler.SignIn)
	kernel.Server.Get(homePath, uiHandler.RedirectHome)
	kernel.Server.Post("/api/sign-in", apiHandler.RequireJsonFromPublicOrigin, limiter.PerIP, limiter.PerAccount(apiHandler.Email), apiHandler.Handle)
}
