package internal

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/features/can_i_deploy"
	"github.com/bidirekt/broker/internal/features/create_environment"
	"github.com/bidirekt/broker/internal/features/create_participant"
	"github.com/bidirekt/broker/internal/features/health"
	"github.com/bidirekt/broker/internal/features/publish_contract"
	"github.com/bidirekt/broker/internal/features/record_deployment"
	"github.com/bidirekt/broker/internal/features/rename_participant"
)

func Run() (*components.Components, error) {
	components, err := components.New()
	if err != nil {
		return nil, err
	}

	create_participant.Register(components)
	create_environment.Register(components)
	publish_contract.Register(components)
	can_i_deploy.Register(components)
	record_deployment.Register(components)
	rename_participant.Register(components)
	health.Register(components)

	return components, nil
}

func Healthcheck() error {
	return health.Check(components.ListenAddr())
}
