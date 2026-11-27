package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const compatibleTypesLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])
`

var compatibleTypesGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func compatibleTypesMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: compatibleTypesLink + sdl}}, compatibleTypesGraph)
	require.NoError(t, readErr)

	catalogued := protocol.Declarations{Resources: declarations.Resources}
	for _, schema := range declarations.Schemas {
		catalogued.AddSchema(schema)
	}

	extension, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	return &model.PersistedContract{
		ParticipantName: name,
		Version:         version,
		Schemas:         fragmentmapper.ToSchemaModels(catalogued),
		Extensions:      map[string]any{"graphql": extension},
	}
}

func compatibleTypesHolder(root, itemType string) string {
	return `
type Query { ` + root + `: Holder }
type Holder @shareable { item: ` + itemType + ` }
interface Node { id: ID! }
type Thing implements Node @shareable { id: ID! }
type Other implements Node @shareable { id: ID! }
`
}

func TestToComposedGraph_AnInterfaceAndItsImplementation_JoinAsTheInterface(t *testing.T) {
	for _, order := range [][]string{{"Node", "Thing"}, {"Thing", "Node"}} {
		composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
			compatibleTypesMember(t, "petstore_api", "1.4.0", compatibleTypesHolder("api", order[0])),
			compatibleTypesMember(t, "petstore_reviews", "2.0.0", compatibleTypesHolder("reviews", order[1])),
		})
		require.Empty(t, errors, order)

		assert.Equal(t, model.Schema{"": {Type: "object"}, "item": {Ref: "Node", Nullable: true}}, composed.Schemas["graphql"]["Holder"], order)
	}
}

func TestToComposedGraph_ANonNullImplementationAndANullableInterface_JoinAsTheNullableInterface(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compatibleTypesMember(t, "petstore_api", "1.4.0", compatibleTypesHolder("api", "Thing!")),
		compatibleTypesMember(t, "petstore_reviews", "2.0.0", compatibleTypesHolder("reviews", "Node")),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "item": {Ref: "Node", Nullable: true}}, composed.Schemas["graphql"]["Holder"])
}

func TestToComposedGraph_TwoImplementationsAndTheirInterface_JoinAsTheInterfaceWhateverTheOrder(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compatibleTypesMember(t, "petstore_api", "1.4.0", compatibleTypesHolder("api", "Thing!")),
		compatibleTypesMember(t, "petstore_orders", "1.0.0", compatibleTypesHolder("orders", "Node!")),
		compatibleTypesMember(t, "petstore_reviews", "2.0.0", compatibleTypesHolder("reviews", "Other!")),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "item": {Ref: "Node"}}, composed.Schemas["graphql"]["Holder"])
}

func TestToComposedGraph_AUnionAndItsMember_JoinAsTheUnion(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compatibleTypesMember(t, "petstore_api", "1.4.0", `
type Query { api: Holder }
type Holder @shareable { item: Result }
union Result = Book
type Book @shareable { id: ID! }
`),
		compatibleTypesMember(t, "petstore_reviews", "2.0.0", `
type Query { reviews: Holder }
type Holder @shareable { item: Book }
type Book @shareable { id: ID! }
`),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "item": {Ref: "Result", Nullable: true}}, composed.Schemas["graphql"]["Holder"])
}

func TestToComposedGraph_ListsOfAnInterfaceAndOfItsImplementation_JoinAsTheListOfTheInterface(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compatibleTypesMember(t, "petstore_api", "1.4.0", compatibleTypesHolder("api", "[Node!]!")),
		compatibleTypesMember(t, "petstore_reviews", "2.0.0", compatibleTypesHolder("reviews", "[Thing!]!")),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{
		"":       {Type: "object"},
		"item":   {Type: "array"},
		"item[]": {Ref: "Node"},
	}, composed.Schemas["graphql"]["Holder"])
}

func TestToComposedGraph_AFieldOfAnImplementationReturningItselfAndTheInterface_JoinsAsTheInterface(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compatibleTypesMember(t, "petstore_api", "1.4.0", `
type Query { api: Animal }
interface Animal { friend: Animal }
type Dog implements Animal @shareable { friend: Dog }
`),
		compatibleTypesMember(t, "petstore_reviews", "2.0.0", `
type Query { reviews: Dog }
interface Animal { friend: Animal }
type Dog implements Animal @shareable { friend: Animal }
`),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "friend": {Ref: "Animal", Nullable: true}}, composed.Schemas["graphql"]["Dog"])
}

func compatibleTypesAnimalHolder(root, itemType string) string {
	return `
type Query { ` + root + `: Holder }
type Holder @shareable { item: ` + itemType + ` }
interface Node { id: ID! }
interface Animal implements Node { id: ID! name: String }
type Dog implements Animal & Node @shareable { id: ID! name: String }
`
}

func TestToComposedGraph_AnInterfaceAndAnInterfaceThatImplementsIt_JoinAsTheWiderOne(t *testing.T) {
	for _, order := range [][]string{{"Node", "Animal"}, {"Animal", "Node"}} {
		composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
			compatibleTypesMember(t, "petstore_api", "1.4.0", compatibleTypesAnimalHolder("api", order[0])),
			compatibleTypesMember(t, "petstore_reviews", "2.0.0", compatibleTypesAnimalHolder("reviews", order[1])),
		})
		require.Empty(t, errors, order)

		assert.Equal(t, model.Schema{"": {Type: "object"}, "item": {Ref: "Node", Nullable: true}}, composed.Schemas["graphql"]["Holder"], order)
		assert.Equal(t, model.Schema{"": {Type: "union"}, "id": {Type: "string", Scalar: "ID"}, "(Dog)": {Ref: "Dog"}}, composed.Schemas["graphql"]["Node"], order)
	}
}

const compatibleTypesUnrelated = `
type Plain @shareable { id: ID! }
interface Loose { id: ID! }
`

func TestToComposedGraph_FieldsOfUnrelatedTypes_DoNotJoin(t *testing.T) {
	cases := []struct {
		name     string
		one      string
		other    string
		expected string
	}{
		{"two scalars", "Int", "String", "petstore_api 1.4.0 resolves Holder.item as Int, but petstore_reviews 2.0.0 as String"},
		{"an interface and an object that doesn't implement it", "Node", "Plain", "petstore_api 1.4.0 resolves Holder.item as Node, but petstore_reviews 2.0.0 as Plain"},
		{"two implementations of one interface", "Thing", "Other", "petstore_api 1.4.0 resolves Holder.item as Thing, but petstore_reviews 2.0.0 as Other"},
		{"a list and its item", "[Thing]", "Node", "petstore_api 1.4.0 resolves Holder.item as [Thing], but petstore_reviews 2.0.0 as Node"},
		{"an interface and another that doesn't implement it", "Node", "Loose", "petstore_api 1.4.0 resolves Holder.item as Node, but petstore_reviews 2.0.0 as Loose"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
				compatibleTypesMember(t, "petstore_api", "1.4.0", compatibleTypesHolder("api", tc.one)+compatibleTypesUnrelated),
				compatibleTypesMember(t, "petstore_reviews", "2.0.0", compatibleTypesHolder("reviews", tc.other)+compatibleTypesUnrelated),
			})

			assert.Equal(t, []string{tc.expected}, errors)
		})
	}
}
