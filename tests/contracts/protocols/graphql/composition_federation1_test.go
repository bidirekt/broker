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

var federation1Graph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func federation1Member(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: sdl}}, federation1Graph)
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

const composedFederation1APISDL = `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const composedFederation1ReviewsSDL = `
extend type Pet @key(fields: "id") {
  id: ID! @external
  name: String!
  reviews: [String!]!
}
`

func TestToComposedGraph_TwoFederation1SubgraphsResolvingTheSameField_Compose(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		federation1Member(t, "petstore_api", "1.4.0", composedFederation1APISDL),
		federation1Member(t, "petstore_reviews", "2.0.0", composedFederation1ReviewsSDL),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{
		"":          {Type: "object"},
		"id":        {Type: "string", Scalar: "ID"},
		"name":      {Type: "string"},
		"reviews":   {Type: "array"},
		"reviews[]": {Type: "string"},
	}, composed.Schemas["graphql"]["Pet"])
}

func TestToComposedGraph_AFederation1SubgraphAndAFederation2OneResolvingTheSameField_FailWithoutShareableInTheFederation2One(t *testing.T) {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		federation1Member(t, "petstore_api", "1.4.0", `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])
`+composedFederation1APISDL),
		federation1Member(t, "petstore_reviews", "2.0.0", composedFederation1ReviewsSDL),
	})

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_reviews 2.0.0 both resolve Pet.name without @shareable"}, errors)
}
