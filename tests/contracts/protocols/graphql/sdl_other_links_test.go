package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const connectorsSDL = `extend schema
  @link(url: "https://specs.apollo.dev/federation/v2.10", import: ["@key"])
  @link(url: "https://specs.apollo.dev/connect/v0.1", import: ["@connect", "@source"])
  @source(name: "petstore", http: {baseURL: "https://petstore.example.com/v1"})

type Query {
  pet(id: ID!): Pet @connect(source: "petstore", http: {GET: "/pets/{$args.id}"}, selection: "id name")
}

type Pet @key(fields: "id") @connect__unlisted(reason: "namespaced") {
  id: ID!
  name: String
}
`

func readLinkedSDL(t *testing.T, sdl string) (protocol.Declarations, graphqlmodel.Extension) {
	t.Helper()

	declarations, err := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: "schema.graphqls", Content: sdl}}, protocol.Declarations{})
	require.NoError(t, err)

	merged, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	return declarations, merged.(graphqlmodel.Extension)
}

func linkedSchemaNames(declarations protocol.Declarations) []string {
	names := make([]string, 0, len(declarations.Schemas))
	for _, declaration := range declarations.Schemas {
		names = append(names, declaration.Name)
	}

	return names
}

func TestSDLFormat_AConnectorsLink_IsReadWithItsDirectivesIgnored(t *testing.T) {
	declarations, merged := readLinkedSDL(t, connectorsSDL)

	assert.Len(t, declarations.Resources, 2)
	assert.Equal(t, []graphqlmodel.Directive{{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true}}, merged.Directives)
}

func TestSDLFormat_ALoneLinkToTheLinkTagOrInaccessibleSpec_IsReadAndIsNoSubgraph(t *testing.T) {
	for name, sdl := range map[string]string{
		"link": `extend schema @link(url: "https://specs.apollo.dev/link/v1.0")

type Query {
  pet: Int
}
`,
		"tag": `extend schema @link(url: "https://specs.apollo.dev/link/v1.0") @link(url: "https://specs.apollo.dev/tag/v0.3", import: ["@tag"])

type Query {
  pet: Int @tag(name: "public")
}
`,
		"inaccessible": `schema @link(url: "https://specs.apollo.dev/inaccessible/v0.2", as: "hidden") {
  query: Query
}

type Query {
  pet: Int @hidden
}
`,
	} {
		declarations, merged := readLinkedSDL(t, sdl)

		assert.NotEmpty(t, declarations.Resources, name)
		assert.Empty(t, merged.Directives, name)
		assert.Empty(t, merged.LinkSource, name)
	}
}

func TestSDLFormat_TheTypesALinkBrings_AreNoSchemas(t *testing.T) {
	declarations, _ := readLinkedSDL(t, `extend schema @link(url: "https://specs.apollo.dev/connect/v0.1", import: ["@connect", "JSONSelection"])

directive @connect(selection: JSONSelection!) repeatable on FIELD_DEFINITION

scalar JSONSelection

scalar connect__URLTemplate

type Query {
  pet: Int @connect(selection: "id")
}
`)

	assert.ElementsMatch(t, []string{"Query", "Query:pet:arguments", "Query:pet:returns"}, linkedSchemaNames(declarations))
}
