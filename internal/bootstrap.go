package internal

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/breakreport"
	graphqlbreakreport "github.com/bidirekt/broker/internal/components/breakreport/protocols/graphql"
	restbreakreport "github.com/bidirekt/broker/internal/components/breakreport/protocols/rest"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/features/can_i_deploy"
	"github.com/bidirekt/broker/internal/features/create_api_key"
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
	"github.com/bidirekt/broker/internal/features/list_user_api_keys"
	"github.com/bidirekt/broker/internal/features/list_users"
	"github.com/bidirekt/broker/internal/features/publish_contract"
	"github.com/bidirekt/broker/internal/features/reactivate_user"
	"github.com/bidirekt/broker/internal/features/record_deployment"
	"github.com/bidirekt/broker/internal/features/rename_environment"
	"github.com/bidirekt/broker/internal/features/revoke_api_key"
	"github.com/bidirekt/broker/internal/features/revoke_user_api_key"
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
	"github.com/bidirekt/broker/internal/repository"
	graphqlrepository "github.com/bidirekt/broker/internal/repository/protocols/graphql"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
)

// Module is one protocol: the pieces every protocol has, and its optional ports, nil when it has none.
type Module struct {
	Protocol           protocol.Protocol
	Hooks              compatibility_checker.ProtocolHooks
	Table              repository.ProtocolTable
	Section            breakreport.ProtocolSection
	FileFormat         contractfiles.FileFormat
	ContractExtensions repository.ContractExtensionStore
	ReservedNames      repository.ReservedNames
}

func RESTModule() Module {
	plugin := restcontracts.New()

	return Module{
		Protocol: plugin,
		Hooks:    plugin,
		Table:    restrepository.New(),
		Section:  restbreakreport.New(),
	}
}

func GraphQLModule() Module {
	plugin := graphqlcontracts.New()
	store := graphqlrepository.New()

	return Module{
		Protocol:           plugin,
		Hooks:              plugin,
		Table:              store,
		Section:            graphqlbreakreport.New(),
		FileFormat:         graphqlcontracts.NewSDLFormat(),
		ContractExtensions: store,
		ReservedNames:      store,
	}
}

func Run() (*components.Components, error) {
	return RunWith([]Module{RESTModule(), GraphQLModule()})
}

func RunWith(modules []Module) (*components.Components, error) {
	var (
		plugins  []protocol.Protocol
		hooks    []compatibility_checker.ProtocolHooks
		tables   []repository.ProtocolTable
		sections []breakreport.ProtocolSection
		formats  []contractfiles.FileFormat
		stores   []repository.ContractExtensionStore
		reserved []repository.ReservedNames
	)

	for _, module := range modules {
		plugins = append(plugins, module.Protocol)
		hooks = append(hooks, module.Hooks)
		tables = append(tables, module.Table)
		sections = append(sections, module.Section)

		if module.FileFormat != nil {
			formats = append(formats, module.FileFormat)
		}

		if module.ContractExtensions != nil {
			stores = append(stores, module.ContractExtensions)
		}

		if module.ReservedNames != nil {
			reserved = append(reserved, module.ReservedNames)
		}
	}

	kernel, err := components.New(sections)
	if err != nil {
		return nil, err
	}

	interpreter := contractfiles.NewInterpreter(protocol.NewRegistry(plugins), formats)
	reportReader := breakreport.NewReader(sections)
	protocols := repository.Protocols{Tables: tables, Extensions: stores}

	// registers /participants/new-participant, which must precede show_participant_versions' /participants/:name
	create_participant.Register(kernel, reserved)
	// registers /environments/new-environment, which must precede show_environment's /environments/:id
	create_environment.Register(kernel)
	publish_contract.Register(kernel, interpreter, protocols)
	validate_contract.Register(kernel, interpreter, protocols, hooks)
	can_i_deploy.Register(kernel, protocols, hooks)
	record_deployment.Register(kernel)
	rename_environment.Register(kernel)
	health.Register(kernel)
	list_participants.Register(kernel)
	list_environments.Register(kernel)
	sign_in.Register(kernel)
	sign_out.Register(kernel)
	update_profile.Register(kernel)
	create_api_key.Register(kernel)
	revoke_api_key.Register(kernel)
	list_users.Register(kernel)
	invite_user.Register(kernel)
	edit_user.Register(kernel)
	generate_link.Register(kernel)
	list_user_api_keys.Register(kernel)
	revoke_user_api_key.Register(kernel)
	deactivate_user.Register(kernel)
	reactivate_user.Register(kernel)
	set_password.Register(kernel)
	setup_admin.Register(kernel)
	show_check.Register(kernel, reportReader)
	list_participant_pages.Register(kernel)
	show_participant_versions.Register(kernel)
	list_participant_deployments.Register(kernel)
	list_participant_checks.Register(kernel, reportReader)
	show_contract_version.Register(kernel, reportReader)
	show_participant_relations.Register(kernel)
	list_environment_pages.Register(kernel)
	show_environment.Register(kernel, reportReader)
	show_compatibility.Register(kernel, reportReader)
	show_overview.Register(kernel, reportReader)

	return kernel, nil
}

func Healthcheck() error {
	return health.Check(config.ListenAddr())
}
