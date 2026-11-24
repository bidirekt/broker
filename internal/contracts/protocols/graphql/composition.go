package graphql

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/guregu/null"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

var rootTypes = []struct {
	operation string
	typeName  string
}{
	{graphqlmodel.QueryOperation, "Query"},
	{graphqlmodel.MutationOperation, "Mutation"},
}

type composition struct {
	subgraphs  []subgraph
	enumUses   map[string]enumUse
	schemas    model.Schemas
	kinds      map[string]string
	fields     map[string][]string
	namedTypes map[string]string
	errors     []compositionError
}

type enumUse struct {
	input  bool
	output bool
}

type compositionError struct {
	coordinate string
	subgraphs  []string
	message    string
}

// ToComposedGraph composes a graph from its subgraphs by the Federation 2 rules. The graph is an in-memory
// contract whose resources are hashed under the graph's name; when it does not compose, the errors come instead,
// ordered by coordinate and then by subgraph.
func ToComposedGraph(graph string, subgraphs []*model.PersistedContract) (*model.PersistedContract, []string) {
	composing := newComposition(subgraphs)
	composing.composeTypes()

	if len(composing.errors) == 0 {
		composing.checkSatisfiability()
	}

	if len(composing.errors) > 0 {
		return nil, composing.sortedErrors()
	}

	return composing.toContract(graph), nil
}

func newComposition(contracts []*model.PersistedContract) *composition {
	subgraphs := make([]subgraph, 0, len(contracts))
	for _, contract := range contracts {
		subgraphs = append(subgraphs, toSubgraph(contract))
	}

	slices.SortFunc(subgraphs, func(a, b subgraph) int {
		return cmp.Compare(a.name, b.name)
	})

	return &composition{
		subgraphs:  subgraphs,
		enumUses:   toEnumUses(subgraphs),
		schemas:    model.Schemas{},
		kinds:      map[string]string{},
		fields:     map[string][]string{},
		namedTypes: map[string]string{},
	}
}

// An enum is an input where an argument or an input field has its type, and an output where any other field has.
func toEnumUses(subgraphs []subgraph) map[string]enumUse {
	uses := map[string]enumUse{}

	for _, member := range subgraphs {
		for coordinate, kind := range member.kinds {
			enum := member.namedTypes[coordinate]
			if member.kinds[enum] != graphqlmodel.EnumKind {
				continue
			}

			use := uses[enum]
			if kind == graphqlmodel.ArgumentKind || member.kinds[ownerOf(coordinate)] == graphqlmodel.InputKind {
				use.input = true
			} else {
				use.output = true
			}

			uses[enum] = use
		}
	}

	return uses
}

func ownerOf(coordinate string) string {
	owner, _, _ := strings.Cut(coordinate, ".")

	return owner
}

func (this *composition) addError(coordinate, message string, subgraphs ...subgraph) {
	names := make([]string, 0, len(subgraphs))
	for _, member := range subgraphs {
		names = append(names, member.name)
	}

	this.errors = append(this.errors, compositionError{coordinate: coordinate, subgraphs: names, message: message})
}

func (this *composition) sortedErrors() []string {
	slices.SortStableFunc(this.errors, func(a, b compositionError) int {
		return cmp.Or(cmp.Compare(a.coordinate, b.coordinate), slices.Compare(a.subgraphs, b.subgraphs))
	})

	messages := make([]string, 0, len(this.errors))
	for _, composed := range this.errors {
		messages = append(messages, composed.message)
	}

	return messages
}

// An element any subgraph marks @inaccessible is out of the graph.
func (this *composition) inaccessible(coordinate string) bool {
	return slices.ContainsFunc(this.subgraphs, func(member subgraph) bool {
		return member.marks(coordinate, graphqlmodel.InaccessibleDirective)
	})
}

func (this *composition) toContract(graph string) *model.PersistedContract {
	contract := &model.PersistedContract{
		ParticipantName: graph,
		Resources:       map[string]model.PersistedResource{},
		Schemas:         map[string]model.Schemas{graphqlmodel.Namespace: this.schemas},
	}

	for _, root := range rootTypes {
		for _, field := range this.fields[root.typeName] {
			arguments := argumentsSchemaName(root.typeName, field)
			if _, taken := this.schemas[arguments]; !taken {
				this.schemas[arguments] = model.Schema{"": {Type: model.ObjectType}}
			}

			returns := returnsSchemaName(root.typeName, field)
			this.schemas[returns] = toReturnsSchema(this.schemas[root.typeName], field)

			addComposedResource(contract, graphqlmodel.Identity{Operation: root.operation, Field: field, Interaction: graphqlmodel.ArgumentsInteraction}, model.Input, arguments)
			addComposedResource(contract, graphqlmodel.Identity{Operation: root.operation, Field: field, Interaction: graphqlmodel.ReturnsInteraction}, model.Output, returns)
		}
	}

	return contract
}

// What a root field returns is its levels in the root type, moved to the top of a schema of their own.
func toReturnsSchema(root model.Schema, field string) model.Schema {
	returns := model.Schema{}

	path := ""
	for level, declared := root[field]; declared; level, declared = root[field+path] {
		level.Arguments = ""
		returns[path] = level
		path += model.ItemsSegment
	}

	return returns
}

func addComposedResource(contract *model.PersistedContract, identity graphqlmodel.Identity, body model.BodyRole, schemaName string) {
	uploaded := model.UploadedResource{Resource: model.Resource{
		ParticipantName: contract.ParticipantName,
		Direction:       model.Provides,
		Protocol:        graphqlmodel.Protocol,
		Body:            body,
		Identity:        identity,
		Bodies:          []model.ResourceBody{{Namespace: graphqlmodel.Namespace, SchemaName: null.StringFrom(schemaName)}},
	}}

	hash := uploaded.ProviderHash()
	contract.Resources[hash] = model.PersistedResource{
		Resource:     uploaded.Resource,
		ProviderHash: hash,
		Schemas:      model.CatalogFor(contract.Schemas, uploaded.Bodies),
	}
}

func (this *composition) typeNames() []string {
	names := map[string]bool{}
	for _, member := range this.subgraphs {
		for name := range member.schemas {
			if member.definesType(name) {
				names[name] = true
			}
		}
	}

	return slices.Sorted(maps.Keys(names))
}

func (this *composition) definersOf(typeName string) []subgraph {
	var definers []subgraph
	for _, member := range this.subgraphs {
		if member.definesType(typeName) {
			definers = append(definers, member)
		}
	}

	return definers
}
