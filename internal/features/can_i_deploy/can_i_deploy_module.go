package can_i_deploy

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	contractRepository := repository.NewContractRepository(kernel.Pool)
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	compatibilityRepository := repository.NewCompatibilityRepository(kernel.Pool)
	compatibilityChecker := compatibility_checker.NewCompatibilityChecker(compatibilityRepository)
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)

	useCase := NewCanIDeployUseCase(
		contractRepository,
		environmentRepository,
		compatibilityRepository,
		compatibilityChecker,
		participantRepository,
	)

	apiHandler := NewCanIDeployApiHandler(useCase)
	kernel.Server.Post("/api/can-i-deploy", apiHandler.Handle)
}
