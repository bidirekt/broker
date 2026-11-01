package internal

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/features/can_i_deploy"
	"github.com/bidirekt/broker/internal/features/create_environment"
	"github.com/bidirekt/broker/internal/features/create_participant"
	"github.com/bidirekt/broker/internal/features/health"
	"github.com/bidirekt/broker/internal/features/list_environments"
	"github.com/bidirekt/broker/internal/features/list_participants"
	"github.com/bidirekt/broker/internal/features/publish_contract"
	"github.com/bidirekt/broker/internal/features/record_deployment"
	"github.com/bidirekt/broker/internal/features/rename_participant"
	"github.com/bidirekt/broker/internal/features/validate_contract"
)

func Run() (*components.Components, error) {
	kernel, err := components.New()
	if err != nil {
		return nil, err
	}

	create_participant.Register(kernel)
	create_environment.Register(kernel)
	publish_contract.Register(kernel)
	validate_contract.Register(kernel)
	can_i_deploy.Register(kernel)
	record_deployment.Register(kernel)
	rename_participant.Register(kernel)
	health.Register(kernel)
	list_participants.Register(kernel)
	list_environments.Register(kernel)

	return kernel, nil
}

func Healthcheck() error {
	return health.Check(components.ListenAddr())
}
