package validate_contract

import (
	"context"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	contractRepository := repository.NewContractRepository(kernel.Pool)
	deploymentRepository := repository.NewDeploymentRepository(kernel.Pool)
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	compatibilityChecker := compatibility_checker.NewCompatibilityChecker(localFilesHaveNoVerdict{})

	useCase := NewValidateContractUseCase(
		contractRepository,
		deploymentRepository,
		environmentRepository,
		participantRepository,
		compatibilityChecker,
	)
	apiHandler := NewValidateContractApiHandler(useCase)
	kernel.Server.Post("/api/contracts/validate", apiHandler.Handle)
}

type localFilesHaveNoVerdict struct{}

func (this localFilesHaveNoVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}
