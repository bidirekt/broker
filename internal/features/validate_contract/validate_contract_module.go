package validate_contract

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
)

func Register(components *components.Components) {
	handler := NewValidateContractHandler(
		repository.NewContractRepository(components.Pool),
		repository.NewDeploymentRepository(components.Pool),
		repository.NewEnvironmentRepository(components.Pool),
		repository.NewParticipantRepository(components.Pool),
	)

	components.Server.Post("/api/contracts/validate", handler.Handle)
}
