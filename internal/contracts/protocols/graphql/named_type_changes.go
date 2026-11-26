package graphql

import (
	"context"
	"strings"

	"github.com/guregu/null"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

type namedTypeChange struct {
	deployed  string
	candidate string
}

// A client's operation declares each variable by its type's name, so a candidate that renames the type of an
// argument or input field a deployed consumer sends breaks that consumer even when the shape stays. The consumer
// declares no names: they are the deployed provider's.
func (this CheckerHooks) toNamedTypeChangePairs(ctx context.Context, contract *model.PersistedContract, environment *model.Environment) []compatibility_checker.ExtraPair {
	if !providesGraphQL(contract) {
		return nil
	}

	deployedVersion, deployedContractID, deployed := this.deployments.CurrentDeploymentInEnv(ctx, contract.ParticipantID, environment.ID)
	if !deployed {
		return nil
	}

	provider := this.contracts.GetContractByIDAndVersion(ctx, deployedContractID, deployedVersion)

	changes := toNamedTypeChanges(provider, contract)
	if len(changes) == 0 {
		return nil
	}

	var pairs []compatibility_checker.ExtraPair

	for _, deployedConsumer := range this.graphs.DeployedConsumers(ctx, environment.ID, contract.ParticipantName) {
		if deployedConsumer.ParticipantID == contract.ParticipantID {
			continue
		}

		consumer := this.contracts.GetContractByIDAndVersion(ctx, deployedConsumer.ContractID, deployedConsumer.Version)
		counterpart := compatibility_checker.Counterpart{
			ParticipantID:      consumer.ParticipantID,
			ParticipantName:    consumer.ParticipantName,
			ParticipantVersion: null.StringFrom(deployedConsumer.Version),
			Role:               model.CounterpartConsumer,
		}

		pairs = append(pairs, toNamedTypeBreakPairs(consumedFrom(consumer, contract.ParticipantName), provider, contract, changes, counterpart, nil)...)
	}

	return pairs
}

func providesGraphQL(contract *model.PersistedContract) bool {
	for _, resource := range contract.Resources {
		if resource.IsProvider() && resource.Protocol == graphqlmodel.Protocol {
			return true
		}
	}

	return false
}

// toNamedTypeChanges are the arguments and input fields whose named type the candidate changes; one the candidate
// drops is no change of name.
func toNamedTypeChanges(deployed, candidate *model.PersistedContract) map[string]namedTypeChange {
	deployedTypes, candidateTypes := typeInfosOf(deployed), typeInfosOf(candidate)
	changes := map[string]namedTypeChange{}

	for coordinate, info := range deployedTypes {
		sent := info.Kind == graphqlmodel.ArgumentKind || info.Kind == graphqlmodel.FieldKind && deployedTypes[ownerOf(coordinate)].Kind == graphqlmodel.InputKind
		if kept, keeps := candidateTypes[coordinate]; sent && keeps && kept.NamedType != info.NamedType {
			changes[coordinate] = namedTypeChange{deployed: info.NamedType, candidate: kept.NamedType}
		}
	}

	return changes
}

func typeInfosOf(contract *model.PersistedContract) map[string]graphqlmodel.TypeInfo {
	extension, _ := contract.Extensions[graphqlmodel.Protocol.String()].(graphqlmodel.Extension)

	infos := make(map[string]graphqlmodel.TypeInfo, len(extension.Types))
	for _, info := range extension.Types {
		infos[info.Coordinate] = info
	}

	return infos
}

// Each break is a pair of its own: a ready break stands for one site of the resource.
func toNamedTypeBreakPairs(
	consumed []model.PersistedResource,
	deployed, candidate *model.PersistedContract,
	changes map[string]namedTypeChange,
	counterpart compatibility_checker.Counterpart,
	details map[string]string,
) []compatibility_checker.ExtraPair {
	var pairs []compatibility_checker.ExtraPair

	for _, resource := range consumed {
		provided, found := deployed.Resources[resource.ProviderHash]
		if !found {
			continue
		}

		changed := candidate.Resources[resource.ProviderHash]

		for _, readyBreak := range toNamedTypeBreaks(&resource, &provided, &changed, changes) {
			pairs = append(pairs, compatibility_checker.ExtraPair{
				Resources:   []model.PersistedResource{resource},
				Counterpart: counterpart,
				ReadyBreak:  readyBreak,
				Details:     details,
			})
		}
	}

	return pairs
}

// namedTypeWalk follows what a consumer resource sends, its arguments and the arguments of the fields it reads,
// through the deployed provider, whose sites are named by their GraphQL coordinates, and through the candidate.
type namedTypeWalk struct {
	consumed *model.PersistedResource
	changes  map[string]namedTypeChange
	visited  map[[2]string]bool
	breaks   []compatibility_checker.ReadyBreak
}

func toNamedTypeBreaks(consumed, provided, changed *model.PersistedResource, changes map[string]namedTypeChange) []compatibility_checker.ReadyBreak {
	walk := &namedTypeWalk{consumed: consumed, changes: changes, visited: map[[2]string]bool{}}
	join := bodyReadings[consumed.Body].Join
	consumer := schemagraph.New(consumed.Schemas).NamingFields(Plugin{}.FieldName).Root(consumed.Bodies, join)
	provider := schemagraph.New(provided.Schemas).NamingFields(Plugin{}.FieldName).Root(provided.Bodies, join)
	candidate := schemagraph.New(changed.Schemas).Root(changed.Bodies, join)

	if consumed.Body == model.Input {
		walk.walkSent(consumer, provider, candidate, "$", "$")
	} else {
		walk.walkRead(consumer, provider, candidate, "$")
	}

	return walk.breaks
}

func (this *namedTypeWalk) walkRead(consumer, provider, candidate schemagraph.Node, path string) {
	if !this.visits(consumer, provider) {
		return
	}

	if consumer.Arguments() != "" {
		this.walkSent(consumer.ArgumentsRoot(schemagraph.Senders), provider.ArgumentsRoot(schemagraph.Senders), candidate.ArgumentsRoot(schemagraph.Senders), path, "$")
	}

	for _, segment := range consumer.Segments() {
		this.walkRead(consumer.Child(segment), provider.Child(segment), candidate.Child(segment), model.ChildPath(path, segment))
	}
}

// A renamed input's fields are no longer the ones the consumer sends: the rename is the break. Where the consumer's
// value no longer fits the candidate, the comparison breaks there already and that break alone stands.
func (this *namedTypeWalk) walkSent(consumer, provider, candidate schemagraph.Node, fieldPath, argumentPath string) {
	if !this.visits(consumer, provider) {
		return
	}

	rules := bodyReadings[model.Input].Rules

	if change, changed := this.changes[provider.Field()]; changed {
		if !candidate.Exists() || !compatibility_checker.Fits(this.consumed, consumer, candidate, rules) {
			return
		}

		this.breaks = append(this.breaks, compatibility_checker.ReadyBreak{
			Reason: model.ReasonPropertyTypeMismatch,
			Details: map[string]string{
				"property":             fieldPath,
				"argument":             strings.TrimPrefix(argumentPath, "$."),
				"consumerName":         this.consumed.ParticipantName,
				"providerName":         this.consumed.ConsumedProvider.String,
				"consumerPropertyType": change.deployed,
				"providerPropertyType": change.candidate,
				"consumerField":        consumer.Field(),
				"providerField":        provider.Field(),
			},
		})

		return
	}

	provider, candidate = compatibility_checker.ShapeRead(provider, consumer, rules), compatibility_checker.ShapeRead(candidate, consumer, rules)

	for _, segment := range consumer.Segments() {
		this.walkSent(consumer.Child(segment), provider.Child(segment), candidate.Child(segment), fieldPath, model.ChildPath(argumentPath, segment))
	}
}

func (this *namedTypeWalk) visits(consumer, provider schemagraph.Node) bool {
	sites := [2]string{consumer.SiteKey(), provider.SiteKey()}
	if !provider.Exists() || this.visited[sites] {
		return false
	}

	this.visited[sites] = true

	return true
}
