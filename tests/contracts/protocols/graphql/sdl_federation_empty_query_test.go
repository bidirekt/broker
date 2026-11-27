package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const entitiesOnlyFederation2SDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Query

type Product @key(fields: "id") {
  id: ID!
  reviews: [Int!]!
}
`

const federationJVMEntitiesOnlySubgraphSDL = `schema {
  query: Query
}

type Product @extends @key(fields : "id") {
  id: ID!
  reviews: [Review!]!
}

type Query

type Review {
  id: ID!
  text: String!
}
`

var inReviewsGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "reviews_graph", GraphSource: "graph.yaml"}}},
}}

func readEmptyQuerySDL(sdl string, yaml protocol.Declarations) (protocol.Declarations, error) {
	return graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: "schema.graphql", Content: sdl}}, yaml)
}

func emptyQuerySchemaNames(declarations protocol.Declarations) []string {
	names := make([]string, 0, len(declarations.Schemas))
	for _, declaration := range declarations.Schemas {
		names = append(names, declaration.Name)
	}

	return names
}

func emptyQueryChecksumInAGraph(t *testing.T, sdl string) string {
	t.Helper()

	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	uploaded, violations, readErr, err := interpreter.ToUploadedContract(
		[]model.ContractFile{{Source: "graph.yaml", Content: "provides:\n  graphql:\n    graph: reviews_graph\n"}, {Source: "schema.graphqls", Content: sdl}},
		&model.Participant{ID: 1, Name: "petstore_reviews"},
		"1.0.0",
	)
	require.NoError(t, err)
	require.NoError(t, readErr)
	require.Empty(t, violations)

	return uploaded.Checksum()
}

func TestSDLFormat_Federation_AnEmptyQueryRoot_IsReadWithNoQueryResource(t *testing.T) {
	declarations, err := readEmptyQuerySDL(entitiesOnlyFederation2SDL, protocol.Declarations{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"Product", "Query"}, emptyQuerySchemaNames(declarations))
	assert.Empty(t, declarations.Resources)
}

func TestSDLFormat_Federation_AnEmptyQueryRootTheSchemaNames_IsReadWithNoQueryResource(t *testing.T) {
	sdl := `schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"]) {
  query: RootQuery
}

type RootQuery

type Product @key(fields: "id") {
  id: ID!
}
`
	declarations, err := readEmptyQuerySDL(sdl, protocol.Declarations{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"Product", "RootQuery"}, emptyQuerySchemaNames(declarations))
	assert.Empty(t, declarations.Resources)
}

func TestSDLFormat_Federation1_AnEmptyQueryRootInAGraph_IsReadWithNoQueryResource(t *testing.T) {
	declarations, err := readEmptyQuerySDL(federationJVMEntitiesOnlySubgraphSDL, inReviewsGraph)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"Product", "Query", "Review"}, emptyQuerySchemaNames(declarations))
	assert.Empty(t, declarations.Resources)
}

func TestSDLFormat_Federation1_AnEmptyQueryRootWithoutAGraph_IsAReadError(t *testing.T) {
	_, err := readEmptyQuerySDL(federationJVMEntitiesOnlySubgraphSDL, protocol.Declarations{})

	require.Error(t, err)
	assert.Equal(t, `schema.graphql:5:15: Undefined directive extends.`, err.Error())
}

func TestSDLFormat_WithoutFederation_AnEmptyQueryRoot_IsAReadError(t *testing.T) {
	_, err := readEmptyQuerySDL("type Query\n\ntype Product {\n  id: ID!\n}\n", protocol.Declarations{})

	require.Error(t, err)
	assert.Equal(t, `schema.graphql:1:6: OBJECT Query: must define one or more fields.`, err.Error())
}

func TestSDLFormat_Federation_ASubgraphWithQueryFields_KeepsTheChecksumItWasPublishedWith(t *testing.T) {
	cases := map[string]struct {
		sdl      string
		checksum string
	}{
		"federation 2": {
			sdl: `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Query {
  product(id: ID!): Product
}

type Product @key(fields: "id") {
  id: ID!
  name: String!
}
`,
			checksum: "f17224b338d2047b940faab249aff759da1b700bcb83a34d67befa515520824b",
		},
		"federation 2 with the query root the schema names": {
			sdl: `schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"]) {
  query: RootQuery
}

type RootQuery {
  product(id: ID!): Product
}

type Product @key(fields: "id") {
  id: ID!
}
`,
			checksum: "8a583aaed4df8e7d78ab289ae1abd3280324c619159bf427ffea9a51a4a2d762",
		},
		"federation 1": {
			sdl: `type Query {
  topReviews: [Review!]!
}

type Review @key(fields: "id") {
  id: ID!
  body: String
}

extend type Product @key(fields: "id") {
  id: ID! @external
  reviews: [Review!]!
}
`,
			checksum: "9785ed4725426431a8e6b3e8ce661f59774e4dc6f7493d36e7b437354d64a581",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.checksum, emptyQueryChecksumInAGraph(t, tc.sdl))
		})
	}
}
