package validate_contract

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(kernel *components.Components) {
	handler := NewValidateContractHandler(
		repository.NewContractRepository(kernel.Pool),
		repository.NewDeploymentRepository(kernel.Pool),
		repository.NewEnvironmentRepository(kernel.Pool),
		repository.NewParticipantRepository(kernel.Pool),
	)

	kernel.Server.Post("/api/contracts/validate", handler.Handle)
}
