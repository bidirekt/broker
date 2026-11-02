package internal

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/features/can_i_deploy"
	"github.com/bidirekt/broker/internal/features/create_environment"
	"github.com/bidirekt/broker/internal/features/create_participant"
	"github.com/bidirekt/broker/internal/features/deactivate_user"
	"github.com/bidirekt/broker/internal/features/edit_user"
	"github.com/bidirekt/broker/internal/features/generate_link"
	"github.com/bidirekt/broker/internal/features/health"
	"github.com/bidirekt/broker/internal/features/invite_user"
	"github.com/bidirekt/broker/internal/features/list_environments"
	"github.com/bidirekt/broker/internal/features/list_participants"
	"github.com/bidirekt/broker/internal/features/list_users"
	"github.com/bidirekt/broker/internal/features/publish_contract"
	"github.com/bidirekt/broker/internal/features/reactivate_user"
	"github.com/bidirekt/broker/internal/features/record_deployment"
	"github.com/bidirekt/broker/internal/features/rename_participant"
	"github.com/bidirekt/broker/internal/features/set_password"
	"github.com/bidirekt/broker/internal/features/sign_in"
	"github.com/bidirekt/broker/internal/features/sign_out"
	"github.com/bidirekt/broker/internal/features/update_profile"
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
	sign_in.Register(kernel)
	sign_out.Register(kernel)
	update_profile.Register(kernel)
	list_users.Register(kernel)
	invite_user.Register(kernel)
	edit_user.Register(kernel)
	generate_link.Register(kernel)
	deactivate_user.Register(kernel)
	reactivate_user.Register(kernel)
	set_password.Register(kernel)

	return kernel, nil
}

func Healthcheck() error {
	return health.Check(config.ListenAddr())
}
