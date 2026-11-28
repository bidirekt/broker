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

const interfaceLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@interfaceObject"])
`

var interfaceGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func interfaceMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: interfaceLink + sdl}}, interfaceGraph)
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

const interfaceAPISDL = `
type Query {
  animals: [Animal!]! @shareable
}

interface Animal {
  name: String!
  age(unit: String): Int!
}

type Dog implements Animal {
  name: String!
  age(unit: String): Int!
}
`

func TestToComposedGraph_TheFieldsOfAnInterface_JoinWithoutShareable(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		interfaceMember(t, "petstore_api", "1.4.0", interfaceAPISDL),
		interfaceMember(t, "petstore_reviews", "2.0.0", `
type Query {
  reviewedAnimals: [Animal!]!
}

interface Animal {
  name: String
  age(unit: String, precise: Boolean): Int!
}

type Cat implements Animal {
  name: String
  age(unit: String, precise: Boolean): Int!
}
`),
	})
	require.Empty(t, errors)

	schemas := composed.Schemas["graphql"]
	assert.Equal(t, model.Schema{
		"":      {Type: "union"},
		"name":  {Type: "string", Nullable: true},
		"age":   {Type: "integer", Arguments: "Animal:age:arguments"},
		"(Cat)": {Ref: "Cat"},
		"(Dog)": {Ref: "Dog"},
	}, schemas["Animal"])
	assert.Equal(t, model.Schema{"": {Type: "object"}, "unit": {Type: "string", Optional: true, Nullable: true}}, schemas["Animal:age:arguments"])
}

func TestToComposedGraph_AnInterfaceFieldOfAnotherTypeInEachSubgraph_DoesNotJoin(t *testing.T) {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		interfaceMember(t, "petstore_api", "1.4.0", interfaceAPISDL),
		interfaceMember(t, "petstore_reviews", "2.0.0", `
type Query {
  animals: [Animal!]! @shareable
}

interface Animal {
  name: Int!
}

type Cat implements Animal {
  name: Int!
}
`),
	})

	assert.Equal(t, []graphqlcontracts.CompositionError{{Code: "FIELD_TYPE_MISMATCH", Message: "petstore_api 1.4.0 resolves Animal.name as String!, but petstore_reviews 2.0.0 as Int!"}}, errors)
}

func TestToComposedGraph_AnInterfaceObject_ComposesAsTheInterfaceWithItsFieldsAdded(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		interfaceMember(t, "petstore_api", "1.4.0", `
type Query {
  media: [Media!]!
}

interface Media @key(fields: "id") {
  id: ID!
  title: String!
}

type Book implements Media @key(fields: "id") {
  id: ID!
  title: String!
}
`),
		interfaceMember(t, "petstore_reviews", "2.0.0", `
type Media @key(fields: "id") @interfaceObject {
  id: ID!
  reviews: [String!]!
}
`),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{
		"":          {Type: "union"},
		"id":        {Type: "string", Scalar: "ID"},
		"title":     {Type: "string"},
		"reviews":   {Type: "array"},
		"reviews[]": {Type: "string"},
		"(Book)":    {Ref: "Book"},
	}, composed.Schemas["graphql"]["Media"])
}

const interfaceObjectAPISDL = `
type Query {
  media: [Media!]!
}

interface Media @key(fields: "id") {
  id: ID!
  title: String!
}

type Book implements Media @key(fields: "id") {
  id: ID!
  title: String!
}

type Movie implements Media @key(fields: "id") {
  id: ID!
  title: String!
  reviews: [String!]
}
`

func TestToComposedGraph_AnInterfaceObject_AddsItsFieldsToEveryImplementationLackingThem(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		interfaceMember(t, "petstore_api", "1.4.0", interfaceObjectAPISDL),
		interfaceMember(t, "petstore_reviews", "2.0.0", `
type Media @key(fields: "id") @interfaceObject {
  id: ID!
  reviews: [String!]!
}
`),
	})
	require.Empty(t, errors)

	schemas := composed.Schemas["graphql"]
	assert.Equal(t, model.Schema{
		"":          {Type: "object"},
		"id":        {Type: "string", Scalar: "ID"},
		"title":     {Type: "string"},
		"reviews":   {Type: "array"},
		"reviews[]": {Type: "string"},
	}, schemas["Book"])
	assert.Equal(t, model.Schema{
		"":          {Type: "object"},
		"id":        {Type: "string", Scalar: "ID"},
		"title":     {Type: "string"},
		"reviews":   {Type: "array", Nullable: true},
		"reviews[]": {Type: "string"},
	}, schemas["Movie"])
}

func TestToComposedGraph_AnInterfaceObjectBehindAKeyThatIsNotResolvable_CannotBeResolved(t *testing.T) {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		interfaceMember(t, "petstore_api", "1.4.0", interfaceObjectAPISDL),
		interfaceMember(t, "petstore_reviews", "2.0.0", `
type Media @key(fields: "id", resolvable: false) @interfaceObject {
  id: ID!
  reviews: [String!]!
}
`),
	})

	assert.Equal(t, []graphqlcontracts.CompositionError{{Code: "SATISFIABILITY_ERROR", Message: "query media.reviews can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Book"}}, errors)
}
