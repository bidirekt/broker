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

const externalLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@external"])
`

var externalGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func externalMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: sdl}}, externalGraph)
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

const externalAPISDL = externalLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

func TestToComposedGraph_AnExternalKeyFieldOfAnExtendType_IsResolvedLikeTheKeyItIs(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		externalMember(t, "petstore_api", "1.4.0", externalAPISDL),
		externalMember(t, "petstore_reviews", "2.0.0", externalLink+`
type Query {
  reviews: [Review!]!
}

type Review {
  body: String!
  pet: Pet
}

extend type Pet @key(fields: "id") {
  id: ID! @external
  reviews: [Review!]!
}
`),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{
		"":          {Type: "object"},
		"id":        {Type: "string", Scalar: "ID"},
		"name":      {Type: "string"},
		"reviews":   {Type: "array"},
		"reviews[]": {Ref: "Review"},
	}, composed.Schemas["graphql"]["Pet"])
}

func TestToComposedGraph_AnExternalFieldOfAnExtensionWithoutKey_IsNotResolvedThere(t *testing.T) {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		externalMember(t, "petstore_api", "1.4.0", externalAPISDL),
		externalMember(t, "petstore_reviews", "2.0.0", externalLink+`
type Query {
  reviews: [Review!]!
}

type Review {
  body: String!
  pet: Pet
}

type Pet @key(fields: "id") {
  reviews: [Review!]!
}

extend type Pet {
  id: ID! @external
}
`),
	})

	assert.Equal(t, []string{
		"query reviews.pet.id can't be resolved: no resolvable @key leads from petstore_reviews 2.0.0 to petstore_api 1.4.0 on Pet",
		"query reviews.pet.name can't be resolved: no resolvable @key leads from petstore_reviews 2.0.0 to petstore_api 1.4.0 on Pet",
	}, errors)
}

const externalPrintedSDL = `schema @link(url: "https://specs.apollo.dev/link/v1.0") @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "FieldSet"]) {
  query: Query
}

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

directive @link(as: String, for: link__Purpose, import: [link__Import], url: String!) repeatable on SCHEMA

union _Entity = Pet

type Pet @key(fields: "id") {
  id: ID!
  %s
}

type Query {
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
  %s
}

type _Service {
  sdl: String!
}

enum link__Purpose {
  EXECUTION
  SECURITY
}

scalar FieldSet

scalar _Any

scalar link__Import
`

func TestToComposedGraph_TwoSubgraphsPrintedWithTheFederationDefinitions_Compose(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		externalMember(t, "petstore_api", "1.4.0", fmt.Sprintf(externalPrintedSDL, "name: String!", "pet(id: ID!): Pet")),
		externalMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(externalPrintedSDL, "rating: Int!", "topPets: [Pet!]!")),
	})
	require.Empty(t, errors)

	var identities []graphqlmodel.Identity
	for _, resource := range composed.Resources {
		identities = append(identities, resource.Identity.(graphqlmodel.Identity))
	}

	assert.ElementsMatch(t, []graphqlmodel.Identity{
		{Operation: "query", Field: "pet", Interaction: "arguments"},
		{Operation: "query", Field: "pet", Interaction: "returns"},
		{Operation: "query", Field: "topPets", Interaction: "arguments"},
		{Operation: "query", Field: "topPets", Interaction: "returns"},
	}, identities)
	assert.NotContains(t, composed.Schemas["graphql"], "_Service")
	assert.NotContains(t, composed.Schemas["graphql"], "_Entity")
}
