package record_deployment

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	participantRepository := repository.NewParticipantRepository(kernel.Pool)
	contractRepository := repository.NewContractRepository(kernel.Pool)
	environmentRepository := repository.NewEnvironmentRepository(kernel.Pool)
	deploymentRepository := repository.NewDeploymentRepository(kernel.Pool)
	handler := NewRecordDeploymentHandler(deploymentRepository, participantRepository, contractRepository, environmentRepository)
	kernel.Server.Post("/api/deployments", handler.Handle)
}
