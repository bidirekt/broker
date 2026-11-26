package graphql_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const compositionLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@external"])
`

var compositionGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func compositionMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: compositionLink + sdl}}, compositionGraph)
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

func composedSchemas(t *testing.T, subgraphs ...*model.PersistedContract) model.Schemas {
	t.Helper()

	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", subgraphs)
	require.Empty(t, errors)

	return composed.Schemas["graphql"]
}

func compositionErrors(subgraphs ...*model.PersistedContract) []string {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", subgraphs)

	return errors
}

const moneyOrdersSDL = `
type Query {
  order(id: ID!): Order
}

type Order @key(fields: "id") {
  id: ID!
  total: Money!
}

type Money %s {
  amount: Int!
}
`

const moneyAPISDL = `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  price: Money!
}

type Money %s {
  amount: Int!
}
`

func TestToComposedGraph_AFieldTwoSubgraphsResolve_ComposesWhenBothMarkItShareable(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(moneyAPISDL, "@shareable")),
		compositionMember(t, "petstore_orders", "1.0.0", fmt.Sprintf(moneyOrdersSDL, "@shareable")),
	)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "amount": {Type: "integer"}}, schemas["Money"])
}

func TestToComposedGraph_AFieldTwoSubgraphsResolve_FailsWithoutShareable(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(moneyAPISDL, "")),
		compositionMember(t, "petstore_orders", "1.0.0", fmt.Sprintf(moneyOrdersSDL, "@shareable")),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable"}, errors)
}

const moneyOrdersWithCurrencySDL = `
type Query {
  order(id: ID!): Order
}

type Order @key(fields: "id") {
  id: ID!
  total: Money!
}

type Money @shareable {
  amount: Int!
  currency: String!
}
`

func TestToComposedGraph_AShareableTypeExtendedWithoutShareable_FailsOnTheFieldsOfTheExtension(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  price: Money!
}

type Money @shareable {
  amount: Int!
}

extend type Money {
  currency: String!
}
`),
		compositionMember(t, "petstore_orders", "1.0.0", moneyOrdersWithCurrencySDL),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.currency without @shareable"}, errors)
}

func TestToComposedGraph_AShareableTypeDeclaredOnce_SharesAllItsFields(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  price: Money!
}

type Money @shareable {
  amount: Int!
  currency: String!
}
`),
		compositionMember(t, "petstore_orders", "1.0.0", moneyOrdersWithCurrencySDL),
	)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "amount": {Type: "integer"}, "currency": {Type: "string"}}, schemas["Money"])
}

func TestToComposedGraph_AnEntityInTwoSubgraphs_SharesTheFieldsOfItsKey(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  name: String!\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Pet @key(fields: \"id\") {\n  id: ID!\n  rating: Int!\n}\n"),
	)

	assert.Equal(t, model.Schema{
		"":       {Type: "object"},
		"id":     {Type: "string", Scalar: "ID"},
		"name":   {Type: "string"},
		"rating": {Type: "integer"},
	}, schemas["Pet"])
}

func TestToComposedGraph_TheMembersOfAUnionAndAnInterface_JoinWithoutShareable(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", `
type Query {
  search: [Result!]! @shareable
  animals: [Animal!]! @shareable
}

union Result = Dog

interface Animal {
  name: String!
}

type Dog implements Animal @shareable {
  name: String!
}
`),
		compositionMember(t, "petstore_reviews", "2.0.0", `
type Query {
  search: [Result!]! @shareable
  animals: [Animal!]! @shareable
}

union Result = Cat

interface Animal {
  name: String!
}

type Cat implements Animal {
  name: String!
}
`),
	)

	assert.Equal(t, model.Schema{"": {Type: "union"}, "(Cat)": {Ref: "Cat"}, "(Dog)": {Ref: "Dog"}}, schemas["Result"])
	assert.Equal(t, model.Schema{"": {Type: "union"}, "name": {Type: "string"}, "(Cat)": {Ref: "Cat"}, "(Dog)": {Ref: "Dog"}}, schemas["Animal"])
}

const agePetSDL = `
type Pet @key(fields: "id") {
  id: ID!
  age: %s @shareable
}
`

func TestToComposedGraph_OutputFieldsOfTheSameType_JoinNullableWhenAnySubgraphIs(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pets: [Pet!]!\n}\n"+fmt.Sprintf(agePetSDL, "[Int!]!")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(agePetSDL, "[Int]!")),
	)

	assert.Equal(t, model.Field{Type: "array"}, schemas["Pet"]["age"])
	assert.Equal(t, model.Field{Type: "integer", Nullable: true}, schemas["Pet"]["age[]"])
}

func TestToComposedGraph_OutputFieldsOfOtherTypes_DoNotJoin(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pets: [Pet!]!\n}\n"+fmt.Sprintf(agePetSDL, "Int")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(agePetSDL, "String")),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 resolves Pet.age as Int, but petstore_reviews 2.0.0 as String"}, errors)
}

func TestToComposedGraph_IDAndString_AreOtherTypesThoughBothAreStrings(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Pet @key(fields: \"id\") {\n  id: String\n}\n"),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 resolves Pet.id as ID, but petstore_reviews 2.0.0 as String"}, errors)
}

const petsWithArgumentsSDL = `
type Query {
  pets%s: [Pet!]! @shareable
}

type Pet @key(fields: "id") {
  id: ID!
}
`

func TestToComposedGraph_Arguments_IntersectAndNonNullWins(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(petsWithArgumentsSDL, "(limit: Int, offset: Int)")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(petsWithArgumentsSDL, "(limit: Int!)")),
	})
	require.Empty(t, errors)

	arguments := composed.Resources[model.Hash("petstore_graph", "graphql", "query", "pets", "arguments")]
	require.Equal(t, "Query:pets:arguments", arguments.Bodies[0].SchemaName.String)
	assert.Equal(t, model.Schema{"": {Type: "object"}, "limit": {Type: "integer"}}, arguments.Schemas["graphql"]["Query:pets:arguments"])
}

func TestToComposedGraph_AnArgumentOneSubgraphRequires_FailsWhereAnotherDoesNotDeclareIt(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(petsWithArgumentsSDL, "(limit: Int!)")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(petsWithArgumentsSDL, "")),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 requires Query.pets(limit:), which petstore_reviews 2.0.0 doesn't declare"}, errors)
}

const filterSDL = `
type Query {
  pets(filter: PetFilter): [Pet!]! @shareable
}

type Pet @key(fields: "id") {
  id: ID!
}

input PetFilter {
  %s
}
`

func TestToComposedGraph_InputFields_IntersectAndNonNullWins(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(filterSDL, "status: String\n  name: String")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(filterSDL, "status: String!")),
	)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "status": {Type: "string"}}, schemas["PetFilter"])
}

func TestToComposedGraph_AnInputFieldOneSubgraphRequires_FailsWhereAnotherDoesNotDeclareIt(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(filterSDL, "status: String\n  species: String!")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(filterSDL, "status: String")),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 requires PetFilter.species, which petstore_reviews 2.0.0 doesn't declare"}, errors)
}

func TestToComposedGraph_AnOutputOnlyEnum_JoinsTheValues(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  status: Status\n}\n\nenum Status {\n  AVAILABLE\n  SOLD\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Pet @key(fields: \"id\") {\n  id: ID!\n  reviewStatus: Status\n}\n\nenum Status {\n  AVAILABLE\n  PENDING\n}\n"),
	)

	assert.Equal(t, model.Schema{"": {Type: "enum", Members: []string{"AVAILABLE", "PENDING", "SOLD"}}}, schemas["Status"])
}

func TestToComposedGraph_AnInputOnlyEnum_IntersectsTheValues(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(petsWithArgumentsSDL, "(status: Status)")+"\nenum Status {\n  AVAILABLE\n  SOLD\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(petsWithArgumentsSDL, "(status: Status)")+"\nenum Status {\n  AVAILABLE\n  PENDING\n}\n"),
	)

	assert.Equal(t, model.Schema{"": {Type: "enum", Members: []string{"AVAILABLE"}}}, schemas["Status"])
}

const statusBothWaysSDL = `
type Query {
  pets(status: Status): [Pet!]! @shareable
}

type Pet @key(fields: "id") {
  id: ID!
  status: Status @shareable
}

enum Status {
  %s
}
`

func TestToComposedGraph_AnEnumUsedBothWays_ComposesWithTheSameValues(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(statusBothWaysSDL, "AVAILABLE\n  SOLD")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(statusBothWaysSDL, "SOLD\n  AVAILABLE")),
	)

	assert.Equal(t, model.Schema{"": {Type: "enum", Members: []string{"AVAILABLE", "SOLD"}}}, schemas["Status"])
}

func TestToComposedGraph_AnEnumUsedBothWays_FailsWithOtherValues(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(statusBothWaysSDL, "AVAILABLE\n  SOLD")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(statusBothWaysSDL, "AVAILABLE\n  PENDING")),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_reviews 2.0.0 declare different values for Status, used as input and output"}, errors)
}

func TestToComposedGraph_WhatIsInaccessible_LeavesTheGraph(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!, debug: Boolean @inaccessible): Pet
  audit: Audit @inaccessible
}

type Pet @key(fields: "id") {
  id: ID!
  status: Status
  secret: String @inaccessible
}

type Audit @inaccessible {
  entries: Int
}

enum Status {
  AVAILABLE
  QUARANTINED @inaccessible
}
`),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Pet @key(fields: \"id\") {\n  id: ID!\n  secret: String @shareable\n}\n"),
	})
	require.Empty(t, errors)

	schemas := composed.Schemas["graphql"]
	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string", Scalar: "ID"}, "status": {Ref: "Status", Nullable: true}}, schemas["Pet"])
	assert.Equal(t, model.Schema{"": {Type: "enum", Members: []string{"AVAILABLE"}}}, schemas["Status"])
	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string", Scalar: "ID"}}, schemas["Query:pet:arguments"])
	assert.NotContains(t, schemas, "Audit")
	assert.NotContains(t, composed.Resources, model.Hash("petstore_graph", "graphql", "query", "audit", "returns"))
}

func TestToComposedGraph_AnExternalField_IsNotResolvedThere(t *testing.T) {
	schemas := composedSchemas(t,
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  weight: Int\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Pet @key(fields: \"id\") {\n  id: ID!\n  weight: Int @external\n}\n"),
	)

	assert.Equal(t, model.Field{Type: "integer", Nullable: true}, schemas["Pet"]["weight"])
}

func TestToComposedGraph_ATypeOfAnotherKind_DisagreesWithTheGenericText(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet: Pet\n}\n\ntype Pet {\n  name: String\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "interface Pet {\n  name: String\n}\n"),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_reviews 2.0.0 disagree on Pet: object in petstore_api, interface in petstore_reviews"}, errors)
}

func TestToComposedGraph_AnArgumentOfAnotherType_DisagreesWithTheGenericText(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(petsWithArgumentsSDL, "(id: ID!)")),
		compositionMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(petsWithArgumentsSDL, "(id: String!)")),
	)

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_reviews 2.0.0 disagree on Query.pets(id:): ID! in petstore_api, String! in petstore_reviews"}, errors)
}

func TestToComposedGraph_SeveralErrors_AreOrderedByCoordinateThenBySubgraph(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_reviews", "2.0.0", "type Query {\n  pets: [Pet!]! @shareable\n}\n\ninterface Pet {\n  id: ID!\n}\n"),
		compositionMember(t, "petstore_orders", "1.0.0", fmt.Sprintf(moneyOrdersSDL, "")),
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pets(limit: Int!): [Pet!]! @shareable\n}\n\ntype Pet {\n  id: ID!\n  price: Money!\n}\n\ntype Money {\n  amount: Int!\n}\n"),
	)

	assert.Equal(t, []string{
		"petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable",
		"petstore_api 1.4.0 and petstore_reviews 2.0.0 disagree on Pet: object in petstore_api, interface in petstore_reviews",
		"petstore_api 1.4.0 requires Query.pets(limit:), which petstore_reviews 2.0.0 doesn't declare",
	}, errors)
}

func TestToComposedGraph_TheGraph_HoldsItsRootFieldsUnderTheGraphName(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Query {\n  reviews: [Review!]!\n}\n\ntype Review {\n  body: String!\n}\n"),
	})
	require.Empty(t, errors)

	assert.Equal(t, int64(0), composed.ID)
	assert.Equal(t, "petstore_graph", composed.ParticipantName)
	assert.Len(t, composed.Resources, 4)

	returns := composed.Resources[model.Hash("petstore_graph", "graphql", "query", "reviews", "returns")]
	assert.Equal(t, model.Provides, returns.Direction)
	assert.Equal(t, model.Output, returns.Body)
	assert.Equal(t, "petstore_graph", returns.ParticipantName)
	assert.Equal(t, "Query:reviews:returns", returns.Bodies[0].SchemaName.String)
	assert.Equal(t, model.Schema{"": {Type: "array"}, "[]": {Ref: "Review"}}, returns.Schemas["graphql"]["Query:reviews:returns"])

	arguments := composed.Resources[model.Hash("petstore_graph", "graphql", "query", "pet", "arguments")]
	assert.Equal(t, model.Input, arguments.Body)
	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string", Scalar: "ID"}}, arguments.Schemas["graphql"]["Query:pet:arguments"])
}

const federationDefinitions = `
directive @link(url: String!, as: String, for: link__Purpose, import: [link__Import]) repeatable on SCHEMA

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

directive @shareable repeatable on OBJECT | FIELD_DEFINITION

scalar link__Import

enum link__Purpose {
  SECURITY
  EXECUTION
}

scalar FieldSet
`

func TestToComposedGraph_SubgraphsThatBothDeclareTheFederationDefinitions_Compose(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", federationDefinitions+fmt.Sprintf(moneyAPISDL, "@shareable")),
		compositionMember(t, "petstore_orders", "1.0.0", federationDefinitions+fmt.Sprintf(moneyOrdersSDL, "@shareable")),
	})
	require.Empty(t, errors)

	assert.Len(t, composed.Resources, 4)
	assert.Equal(t, model.Schema{"": {Type: "object"}, "amount": {Type: "integer"}}, composed.Schemas["graphql"]["Money"])
}

func TestToComposedGraph_ARootTypeTheSchemaNamesOtherwise_JoinsTheQueryRoot(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  name: String!\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Query {\n  reviews: [Review!]!\n}\n\ntype Review {\n  body: String!\n}\n"),
	})
	require.Empty(t, errors)

	assert.Len(t, composed.Resources, 4)
	assert.NotContains(t, composed.Schemas["graphql"], "RootQuery")

	arguments := composed.Resources[model.Hash("petstore_graph", "graphql", "query", "pet", "arguments")]
	assert.Equal(t, "Query:pet:arguments", arguments.Bodies[0].SchemaName.String)
	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string", Scalar: "ID"}}, arguments.Schemas["graphql"]["Query:pet:arguments"])

	returns := composed.Resources[model.Hash("petstore_graph", "graphql", "query", "pet", "returns")]
	assert.Equal(t, model.Schema{"": {Ref: "Pet", Nullable: true}}, returns.Schemas["graphql"]["Query:pet:returns"])
}

func TestToComposedGraph_AMutationRootTheSchemaNamesOtherwise_GivesMutations(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", "schema {\n  query: RootQuery\n  mutation: RootMutation\n}\n\ntype RootQuery {\n  pet(id: ID!): Pet\n}\n\ntype RootMutation {\n  adoptPet(id: ID!): Pet!\n}\n\ntype Pet {\n  id: ID!\n}\n"),
	})
	require.Empty(t, errors)

	assert.Len(t, composed.Resources, 4)
	assert.Contains(t, composed.Resources, model.Hash("petstore_graph", "graphql", "mutation", "adoptPet", "returns"))
}

func TestToComposedGraph_AQueryRootThatIsAlsoTheMutationRoot_GivesQueries(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", "schema {\n  query: Query\n  mutation: Query\n}\n\ntype Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet {\n  id: ID!\n}\n"),
	})
	require.Empty(t, errors)

	assert.Len(t, composed.Resources, 2)
	assert.Contains(t, composed.Resources, model.Hash("petstore_graph", "graphql", "query", "pet", "returns"))
	assert.Contains(t, composed.Resources, model.Hash("petstore_graph", "graphql", "query", "pet", "arguments"))
	assert.Contains(t, composed.Schemas["graphql"], "Query")
	assert.NotContains(t, composed.Schemas["graphql"], "Mutation")
}

func TestToComposedGraph_ErrorsOnOneCoordinate_AreOrderedBySubgraph(t *testing.T) {
	errors := compositionErrors(
		compositionMember(t, "petstore_reviews", "2.0.0", "type Review {\n  price: Money!\n}\n\ntype Money {\n  amount: Int!\n}\n"),
		compositionMember(t, "petstore_orders", "1.0.0", "type Query {\n  order(id: ID!): Order\n}\n\ntype Order @key(fields: \"id\") {\n  id: ID!\n  total: Money!\n}\n\ntype Money @shareable {\n  amount: String!\n}\n"),
		compositionMember(t, "petstore_api", "1.4.0", fmt.Sprintf(moneyAPISDL, "@shareable")),
	)

	assert.Equal(t, []string{
		"petstore_api 1.4.0 resolves Money.amount as Int!, but petstore_orders 1.0.0 as String!",
		"petstore_api 1.4.0 and petstore_reviews 2.0.0 both resolve Money.amount without @shareable",
		"petstore_orders 1.0.0 and petstore_reviews 2.0.0 both resolve Money.amount without @shareable",
	}, errors)
}

func TestToComposedGraph_TheGraph_KeepsTheNamedTypeOfEveryArgumentAndInputField(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		compositionMember(t, "petstore_api", "1.4.0", "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Mutation {\n  addPet(input: AddPetInput!): Pet\n}\n\ninput AddPetInput {\n  ownerId: ID!\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"),
		compositionMember(t, "petstore_reviews", "2.0.0", "type Pet @key(fields: \"id\") {\n  id: ID!\n  reviews(by: ID): [String!]!\n}\n"),
	})
	require.Empty(t, errors)

	types := composed.Extensions["graphql"].(graphqlmodel.Extension).Types

	assert.Contains(t, types, graphqlmodel.TypeInfo{Coordinate: "Query.pet(id:)", Kind: "argument", NamedType: "ID"})
	assert.Contains(t, types, graphqlmodel.TypeInfo{Coordinate: "Mutation.addPet(input:)", Kind: "argument", NamedType: "AddPetInput"})
	assert.Contains(t, types, graphqlmodel.TypeInfo{Coordinate: "AddPetInput", Kind: "input"})
	assert.Contains(t, types, graphqlmodel.TypeInfo{Coordinate: "AddPetInput.ownerId", Kind: "field", NamedType: "ID"})
	assert.Contains(t, types, graphqlmodel.TypeInfo{Coordinate: "Pet.reviews(by:)", Kind: "argument", NamedType: "ID"})
	assert.Contains(t, types, graphqlmodel.TypeInfo{Coordinate: "Pet.reviews", Kind: "field", NamedType: "String"})
}
