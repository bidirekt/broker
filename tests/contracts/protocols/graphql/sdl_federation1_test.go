package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const federation1ReviewsSDL = `extend type Query {
  topReviews: [Review!]!
}

type Review @key(fields: "id") {
  id: ID!
  body: String
  pet: Pet @provides(fields: "name")
}

extend type Pet @key(fields: "id") {
  id: ID! @external
  name: String @external
  reviews: [Review!]! @requires(fields: "name") @tag(name: "public")
}
`

var inPetstoreGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func readSubgraphSDL(sdl string, yaml protocol.Declarations) (protocol.Declarations, error) {
	return graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: "reviews.graphqls", Content: sdl}}, yaml)
}

func subgraphSchemaNames(declarations protocol.Declarations) []string {
	names := make([]string, 0, len(declarations.Schemas))
	for _, declaration := range declarations.Schemas {
		names = append(names, declaration.Name)
	}

	return names
}

func TestSDLFormat_Federation1_ASubgraphWithoutALink_IsReadWithTheFederation1Directives(t *testing.T) {
	declarations, err := readSubgraphSDL(federation1ReviewsSDL, inPetstoreGraph)
	require.NoError(t, err)

	merged, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	extension := merged.(graphqlmodel.Extension)
	assert.True(t, extension.Federation1)
	assert.Empty(t, extension.LinkSource)
	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet.name", Directive: "external", Resolvable: true},
		{Coordinate: "Review", Directive: "key", Fields: "id", Resolvable: true},
	}, extension.Directives)
	assert.ElementsMatch(t, []string{"Pet", "Query", "Review", "Query:topReviews:arguments", "Query:topReviews:returns"}, subgraphSchemaNames(declarations))
}

func TestSDLFormat_Federation1_WithoutAGraph_TheSameSDLIsAReadError(t *testing.T) {
	_, err := readSubgraphSDL(federation1ReviewsSDL, protocol.Declarations{})

	require.Error(t, err)
	assert.Equal(t, `reviews.graphqls:12:12: Undefined directive external.`, err.Error())
}

func TestSDLFormat_Federation1_AGraphOutsideTheNameFormat_IsRefusedAtTheGraphNotReadWithoutIt(t *testing.T) {
	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	_, violations, readErr := interpreter.ToViolations([]model.ContractFile{
		{Source: "graph.yaml", Content: "provides:\n  graphql:\n    graph: petstore-graph\n"},
		{Source: "reviews.graphqls", Content: federation1ReviewsSDL},
	})

	require.NoError(t, readErr)
	assert.Equal(t, []violation.Violation{{
		ErrorCode: "graph.name_syntax",
		Path:      "provides;graphql;graph",
		Source:    "graph.yaml",
		Details:   map[string]any{"graph": "petstore-graph", "error": "must be snake_case"},
	}}, violations)
}

func TestSDLFormat_Federation1_ASubgraphWithAFederation2Link_IsNoFederation1Subgraph(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
}
`
	declarations, err := readSubgraphSDL(sdl, inPetstoreGraph)
	require.NoError(t, err)

	merged, _ := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	assert.False(t, merged.(graphqlmodel.Extension).Federation1)
}

func TestSDLFormat_AnExtendOfATypeNoFileDefines_IsReadInAGraph(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

extend type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
}
`
	declarations, err := readSubgraphSDL(sdl, inPetstoreGraph)

	require.NoError(t, err)
	assert.Len(t, declarations.Resources, 2)
}

func TestSDLFormat_Federation1_ThePrintedFederationDefinitions_AreNoSchemasNorResources(t *testing.T) {
	sdl := `directive @key(fields: _FieldSet!) repeatable on OBJECT | INTERFACE

scalar _FieldSet

scalar _Any

union _Entity = Pet

type _Service {
  sdl: String
}

type Query {
  pet: Pet
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
}

type Pet @key(fields: "id") {
  id: ID!
}
`
	declarations, err := readSubgraphSDL(sdl, inPetstoreGraph)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"Pet", "Query", "Query:pet:arguments", "Query:pet:returns"}, subgraphSchemaNames(declarations))
	assert.Len(t, declarations.Resources, 2)
}

func TestSDLFormat_Federation1_AnExternalType_MakesNoFieldExternal(t *testing.T) {
	sdl := `type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
  dimensions: Dimensions @external
}

type Dimensions @external {
  height: Int
}
`
	declarations, err := readSubgraphSDL(sdl, inPetstoreGraph)
	require.NoError(t, err)

	merged, _ := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet.dimensions", Directive: "external", Resolvable: true},
	}, merged.(graphqlmodel.Extension).Directives)
}
