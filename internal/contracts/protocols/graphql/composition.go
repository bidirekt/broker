package graphql

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/guregu/null"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

type rootType struct {
	operation string
	typeName  string
	of        func(graphqlmodel.Roots) string
}

// The query root comes first: a type that is both roots is the query root, as the SDL reads it.
var rootTypes = []rootType{
	{graphqlmodel.QueryOperation, "Query", func(roots graphqlmodel.Roots) string { return roots.Query }},
	{graphqlmodel.MutationOperation, "Mutation", func(roots graphqlmodel.Roots) string { return roots.Mutation }},
}

type composition struct {
	subgraphs                []subgraph
	enumUses                 map[string]enumUse
	schemas                  model.Schemas
	kinds                    map[string]string
	fields                   map[string][]string
	namedTypes               map[string]string
	interfaceObjectResolvers map[string][]subgraph
	errors                   []compositionError
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

	// A step runs only when the ones before it found no error: a root that takes a type's name garbles the schemas.
	for _, step := range []func(){composing.checkRootNames, composing.compose, composing.checkSatisfiability} {
		if len(composing.errors) == 0 {
			step()
		}
	}

	if len(composing.errors) > 0 {
		return nil, composing.sortedErrors()
	}

	return composing.toContract(graph), nil
}

func (this *composition) compose() {
	this.takeOverrides()
	this.composeTypes()
	this.addInterfaceObjectFields()
	this.checkReferences()
}

// A root the SDL names otherwise next to a type of its own with the root's default name would join that type.
func (this *composition) checkRootNames() {
	for _, member := range this.subgraphs {
		for _, root := range member.usedRoots {
			this.addError(root.typeName, fmt.Sprintf("%s has a type named %s, but its %s root is %s (ROOT_%s_USED)",
				member.label, root.typeName, root.operation, root.of(member.roots), strings.ToUpper(root.operation)), member)
		}
	}
}

// The graph has the roots some subgraph declares: a type named Mutation is no mutation root on its own.
func (this *composition) roots() []rootType {
	return slices.DeleteFunc(slices.Clone(rootTypes), func(root rootType) bool {
		return !slices.ContainsFunc(this.subgraphs, func(member subgraph) bool { return root.of(member.roots) != "" })
	})
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
		subgraphs:                subgraphs,
		enumUses:                 toEnumUses(subgraphs),
		schemas:                  model.Schemas{},
		kinds:                    map[string]string{},
		fields:                   map[string][]string{},
		namedTypes:               map[string]string{},
		interfaceObjectResolvers: map[string][]subgraph{},
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

// takeOverrides leaves each field a subgraph takes over with @override to that subgraph alone. An @override from a
// subgraph that is not a member, as one named after the subgraph's name in Apollo, takes the field from every other
// member that resolves it without overriding it too.
func (this *composition) takeOverrides() {
	for _, member := range this.subgraphs {
		for _, coordinate := range slices.Sorted(maps.Keys(member.directives)) {
			for _, directive := range member.directives[coordinate] {
				if directive.Directive == graphqlmodel.OverrideDirective {
					this.takeOverride(member, coordinate, directive.From)
				}
			}
		}
	}
}

func (this *composition) takeOverride(member subgraph, coordinate, from string) {
	if from == member.name {
		this.addError(coordinate, fmt.Sprintf("%s overrides %s from itself", member.label, coordinate), member)

		return
	}

	index := slices.IndexFunc(this.subgraphs, func(other subgraph) bool { return other.name == from })
	if index < 0 {
		for _, other := range this.subgraphs {
			if other.name != member.name && !other.marks(coordinate, graphqlmodel.OverrideDirective) {
				other.overridden[coordinate] = true
			}
		}

		return
	}

	origin := this.subgraphs[index]
	if origin.marks(coordinate, graphqlmodel.OverrideDirective) {
		this.addError(coordinate, fmt.Sprintf("%s overrides %s from %s, which overrides it too", member.label, coordinate, origin.name), member, origin)

		return
	}

	origin.overridden[coordinate] = true
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

	for _, root := range this.roots() {
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
