package graphql

import (
	"context"
	"maps"
	"slices"

	"github.com/guregu/null"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// candidateVersion labels a candidate that has no version yet, the one the validate checks.
const candidateVersion = "candidate"

type GraphLoader interface {
	GraphNamed(ctx context.Context, name string) bool
	DeployedMembers(ctx context.Context, environmentID int64, graph string) []graphqlmodel.DeployedContract
	DeployedConsumers(ctx context.Context, environmentID int64, graph string) []graphqlmodel.DeployedContract
	DeployedGraph(ctx context.Context, environmentID, participantID int64) string
}

type ContractLoader interface {
	GetContractByIDAndVersion(ctx context.Context, contractID int64, version string) *model.PersistedContract
}

// CheckerHooks are the GraphQL checker hooks that also find the graphs a contract consumes or changes, composed
// from what is deployed in the environment.
type CheckerHooks struct {
	Plugin
	graphs    GraphLoader
	contracts ContractLoader
}

func NewCheckerHooks(graphs GraphLoader, contracts ContractLoader) CheckerHooks {
	return CheckerHooks{graphs: graphs, contracts: contracts}
}

func (this CheckerHooks) ExtraCounterparts(
	ctx context.Context,
	contract *model.PersistedContract,
	environment *model.Environment,
) compatibility_checker.ExtraPairs {
	pairs := compatibility_checker.ExtraPairs{Providers: map[string]bool{}}
	graphs := newGraphsInCheck(this, contract, environment)
	candidateGraph := graphOf(contract)

	for _, graph := range consumedProviders(contract) {
		if graph != candidateGraph && !this.graphs.GraphNamed(ctx, graph) {
			continue
		}

		pairs.Providers[graph] = true
		pairs.Pairs = append(pairs.Pairs, toGraphPair(contract, graphs.candidate(ctx, graph)))
	}

	for _, graph := range this.affectedGraphs(ctx, contract, environment) {
		pairs.Pairs = append(pairs.Pairs, this.toGraphConsumerPairs(ctx, graphs, graph)...)
	}

	return pairs
}

func toGraphPair(contract *model.PersistedContract, graph composedGraph) compatibility_checker.ExtraPair {
	counterpart := compatibility_checker.Counterpart{ParticipantName: graph.name, Role: model.CounterpartProvider}

	if len(graph.members) == 0 {
		return compatibility_checker.ExtraPair{
			Resources:   consumedReturns(contract, graph.name),
			Counterpart: counterpart,
			ReadyBreak: compatibility_checker.ReadyBreak{
				Reason:  model.ReasonProviderResourceNotDeployedInEnvironment,
				Details: map[string]string{graphqlmodel.GraphKey: graph.name},
			},
		}
	}

	counterpart.Members = graph.members

	if graph.contract == nil {
		return compatibility_checker.ExtraPair{
			Resources:   consumedReturns(contract, graph.name),
			Counterpart: counterpart,
			ReadyBreak:  graph.notComposable(),
		}
	}

	return compatibility_checker.ExtraPair{
		Resources:   consumedFrom(contract, graph.name),
		Contract:    graph.contract,
		Counterpart: counterpart,
	}
}

// The graphs a subgraph's check changes are the one its candidate declares and the one its deployed version is in.
func (this CheckerHooks) affectedGraphs(ctx context.Context, contract *model.PersistedContract, environment *model.Environment) []string {
	graphs := map[string]bool{}

	for _, graph := range []string{graphOf(contract), this.graphs.DeployedGraph(ctx, environment.ID, contract.ParticipantID)} {
		if graph != "" {
			graphs[graph] = true
		}
	}

	return slices.Sorted(maps.Keys(graphs))
}

// Each deployed consumer of the graph, other than the participant under check, is compared with the candidate
// composition, and only the breaks the base composition does not have count: a break already there is the
// consumer's to see in its own check.
func (this CheckerHooks) toGraphConsumerPairs(ctx context.Context, graphs *graphsInCheck, graph string) []compatibility_checker.ExtraPair {
	var pairs []compatibility_checker.ExtraPair

	for _, deployed := range this.graphs.DeployedConsumers(ctx, graphs.environment.ID, graph) {
		if deployed.ParticipantID == graphs.contract.ParticipantID {
			continue
		}

		consumer := this.contracts.GetContractByIDAndVersion(ctx, deployed.ContractID, deployed.Version)
		if len(consumedFrom(consumer, graph)) == 0 {
			continue
		}

		candidate, base := graphs.candidate(ctx, graph), graphs.base(ctx, graph)
		counterpart := compatibility_checker.Counterpart{
			ParticipantID:      consumer.ParticipantID,
			ParticipantName:    consumer.ParticipantName,
			ParticipantVersion: null.StringFrom(deployed.Version),
			Role:               model.CounterpartConsumer,
		}

		switch {
		case candidate.contract == nil && base.contract != nil:
			pairs = append(pairs, compatibility_checker.ExtraPair{
				Resources:   consumedReturns(consumer, graph),
				Counterpart: counterpart,
				ReadyBreak:  candidate.notComposable(),
			})
		case candidate.contract != nil:
			pairs = append(pairs, compatibility_checker.ExtraPair{
				Resources:   consumedFrom(consumer, graph),
				Contract:    candidate.contract,
				Counterpart: counterpart,
				Base:        base.contract,
				Details:     map[string]string{graphqlmodel.GraphKey: graph},
			})
		}
	}

	return pairs
}

// graphsInCheck composes each graph once per check: the candidate composition serves the participant as a consumer
// of the graph and as one of its subgraphs.
type graphsInCheck struct {
	hooks       CheckerHooks
	contract    *model.PersistedContract
	environment *model.Environment
	deployed    map[string][]*model.PersistedContract
	candidates  map[string]composedGraph
	bases       map[string]composedGraph
}

func newGraphsInCheck(hooks CheckerHooks, contract *model.PersistedContract, environment *model.Environment) *graphsInCheck {
	return &graphsInCheck{
		hooks:       hooks,
		contract:    contract,
		environment: environment,
		deployed:    map[string][]*model.PersistedContract{},
		candidates:  map[string]composedGraph{},
		bases:       map[string]composedGraph{},
	}
}

// The candidate composition has the candidate in place of the participant's deployed version, and has the
// participant only when the candidate declares the graph.
func (this *graphsInCheck) candidate(ctx context.Context, graph string) composedGraph {
	if composed, done := this.candidates[graph]; done {
		return composed
	}

	members := slices.DeleteFunc(slices.Clone(this.deployedMembers(ctx, graph)), func(member *model.PersistedContract) bool {
		return member.ParticipantID == this.contract.ParticipantID
	})

	if graphOf(this.contract) == graph {
		candidate := *this.contract
		if candidate.Version == "" {
			candidate.Version = candidateVersion
		}

		members = append(members, &candidate)
	}

	this.candidates[graph] = toComposedGraph(graph, members)

	return this.candidates[graph]
}

// The base composition is the graph as deployed, with the participant's deployed version when it is in the graph.
func (this *graphsInCheck) base(ctx context.Context, graph string) composedGraph {
	if composed, done := this.bases[graph]; done {
		return composed
	}

	this.bases[graph] = toComposedGraph(graph, this.deployedMembers(ctx, graph))

	return this.bases[graph]
}

func (this *graphsInCheck) deployedMembers(ctx context.Context, graph string) []*model.PersistedContract {
	if members, loaded := this.deployed[graph]; loaded {
		return members
	}

	members := []*model.PersistedContract{}
	for _, deployed := range this.hooks.graphs.DeployedMembers(ctx, this.environment.ID, graph) {
		members = append(members, this.hooks.contracts.GetContractByIDAndVersion(ctx, deployed.ContractID, deployed.Version))
	}

	this.deployed[graph] = members

	return members
}

// composedGraph is a graph composed from its members: the composed contract, or the failures when it does not
// compose.
type composedGraph struct {
	name     string
	members  map[string]string
	contract *model.PersistedContract
	failures []string
}

func toComposedGraph(name string, members []*model.PersistedContract) composedGraph {
	versions := make(map[string]string, len(members))
	for _, member := range members {
		versions[member.ParticipantName] = member.Version
	}

	contract, failures := ToComposedGraph(name, members)

	return composedGraph{name: name, members: versions, contract: contract, failures: failures}
}

func (this composedGraph) notComposable() compatibility_checker.ReadyBreak {
	return compatibility_checker.ReadyBreak{
		Reason:  graphqlmodel.ReasonProviderGraphNotComposable,
		Details: map[string]string{graphqlmodel.GraphKey: this.name, graphqlmodel.CauseKey: this.failures[0]},
	}
}

func graphOf(contract *model.PersistedContract) string {
	extension, _ := contract.Extensions[graphqlmodel.Protocol.String()].(graphqlmodel.Extension)

	return extension.Graph
}

func consumedProviders(contract *model.PersistedContract) []string {
	providers := map[string]bool{}
	for _, resource := range consumedGraphQL(contract) {
		providers[resource.ConsumedProvider.String] = true
	}

	return slices.Sorted(maps.Keys(providers))
}

func consumedFrom(contract *model.PersistedContract, provider string) []model.PersistedResource {
	return slices.DeleteFunc(consumedGraphQL(contract), func(resource model.PersistedResource) bool {
		return resource.ConsumedProvider.String != provider
	})
}

// A break that stands for the whole field is reported once, under what the field returns.
func consumedReturns(contract *model.PersistedContract, provider string) []model.PersistedResource {
	return slices.DeleteFunc(consumedFrom(contract, provider), func(resource model.PersistedResource) bool {
		return resource.Identity.(graphqlmodel.Identity).Interaction != graphqlmodel.ReturnsInteraction
	})
}

func consumedGraphQL(contract *model.PersistedContract) []model.PersistedResource {
	var consumed []model.PersistedResource
	for _, resource := range contract.Resources {
		if resource.IsConsumer() && !resource.Removed && resource.Protocol == graphqlmodel.Protocol {
			consumed = append(consumed, resource)
		}
	}

	return consumed
}
