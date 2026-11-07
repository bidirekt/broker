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
	"github.com/bidirekt/broker/internal/features/list_environment_pages"
	"github.com/bidirekt/broker/internal/features/list_environments"
	"github.com/bidirekt/broker/internal/features/list_participant_checks"
	"github.com/bidirekt/broker/internal/features/list_participant_deployments"
	"github.com/bidirekt/broker/internal/features/list_participant_pages"
	"github.com/bidirekt/broker/internal/features/list_participants"
	"github.com/bidirekt/broker/internal/features/list_users"
	"github.com/bidirekt/broker/internal/features/publish_contract"
	"github.com/bidirekt/broker/internal/features/reactivate_user"
	"github.com/bidirekt/broker/internal/features/record_deployment"
	"github.com/bidirekt/broker/internal/features/rename_environment"
	"github.com/bidirekt/broker/internal/features/set_password"
	"github.com/bidirekt/broker/internal/features/setup_admin"
	"github.com/bidirekt/broker/internal/features/show_check"
	"github.com/bidirekt/broker/internal/features/show_compatibility"
	"github.com/bidirekt/broker/internal/features/show_contract_version"
	"github.com/bidirekt/broker/internal/features/show_environment"
	"github.com/bidirekt/broker/internal/features/show_overview"
	"github.com/bidirekt/broker/internal/features/show_participant_relations"
	"github.com/bidirekt/broker/internal/features/show_participant_versions"
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

	// registers /participants/new-participant, which must precede show_participant_versions' /participants/:name
	create_participant.Register(kernel)
	// registers /environments/new-environment, which must precede show_environment's /environments/:id
	create_environment.Register(kernel)
	publish_contract.Register(kernel)
	validate_contract.Register(kernel)
	can_i_deploy.Register(kernel)
	record_deployment.Register(kernel)
	rename_environment.Register(kernel)
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
	setup_admin.Register(kernel)
	show_check.Register(kernel)
	list_participant_pages.Register(kernel)
	show_participant_versions.Register(kernel)
	list_participant_deployments.Register(kernel)
	list_participant_checks.Register(kernel)
	show_contract_version.Register(kernel)
	show_participant_relations.Register(kernel)
	list_environment_pages.Register(kernel)
	show_environment.Register(kernel)
	show_compatibility.Register(kernel)
	show_overview.Register(kernel)

	return kernel, nil
}

func Healthcheck() error {
	return health.Check(config.ListenAddr())
}
