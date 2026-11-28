package show_graph

import (
	"cmp"
	"context"
	"slices"

	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	graphqlrepository "github.com/bidirekt/broker/internal/repository/protocols/graphql"
)

type ShowGraphStatus int

const (
	GraphFound ShowGraphStatus = iota
	GraphNotFound
)

type ShowGraphInput struct {
	Name string
}

type ComposedEnvironment struct {
	Environment model.Environment
	Members     []*model.PersistedContract
	Errors      []string
}

type ShowGraphOutput struct {
	Status       ShowGraphStatus
	Name         string
	Environments []ComposedEnvironment
}

type ShowGraphUseCase struct {
	graphs                graphqlrepository.GraphLoader
	environmentRepository *repository.EnvironmentRepository
	contractRepository    *repository.ContractRepository
}

func NewShowGraphUseCase(
	graphs graphqlrepository.GraphLoader,
	environmentRepository *repository.EnvironmentRepository,
	contractRepository *repository.ContractRepository,
) *ShowGraphUseCase {
	return &ShowGraphUseCase{
		graphs:                graphs,
		environmentRepository: environmentRepository,
		contractRepository:    contractRepository,
	}
}

func (this *ShowGraphUseCase) Execute(ctx context.Context, input ShowGraphInput) ShowGraphOutput {
	if !this.graphs.GraphNamed(ctx, input.Name) {
		return ShowGraphOutput{Status: GraphNotFound}
	}

	environments := this.environmentRepository.List(ctx)
	composed := make([]ComposedEnvironment, 0, len(environments))
	for _, environment := range environments {
		composed = append(composed, this.toComposedEnvironment(ctx, input.Name, environment))
	}

	return ShowGraphOutput{Status: GraphFound, Name: input.Name, Environments: composed}
}

func (this *ShowGraphUseCase) toComposedEnvironment(ctx context.Context, graph string, environment model.Environment) ComposedEnvironment {
	var members []*model.PersistedContract
	for _, deployed := range this.graphs.DeployedMembers(ctx, environment.ID, graph) {
		members = append(members, this.contractRepository.GetContractByIDAndVersion(ctx, deployed.ContractID, deployed.Version))
	}

	slices.SortFunc(members, func(a, b *model.PersistedContract) int {
		return cmp.Compare(a.ParticipantName, b.ParticipantName)
	})

	composed := ComposedEnvironment{Environment: environment, Members: members}
	if len(members) > 0 {
		_, errors := graphqlcontracts.ToComposedGraph(graph, members)
		composed.Errors = graphqlcontracts.CompositionMessages(errors)
	}

	return composed
}
