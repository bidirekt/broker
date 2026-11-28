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

const codesLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@external", "@override"])
`

func codesMember(t *testing.T, name, sdl string) *model.PersistedContract {
	t.Helper()

	graph := protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
		"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
	}}

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: codesLink + sdl}}, graph)
	require.NoError(t, readErr)

	catalogued := protocol.Declarations{Resources: declarations.Resources}
	for _, schema := range declarations.Schemas {
		catalogued.AddSchema(schema)
	}

	extension, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	return &model.PersistedContract{
		ParticipantName: name,
		Version:         "1.0.0",
		Schemas:         fragmentmapper.ToSchemaModels(catalogued),
		Extensions:      map[string]any{"graphql": extension},
	}
}

func TestToComposedGraph_EachRule_FailsWithTheCodeOfItsApolloRule(t *testing.T) {
	cases := map[string]struct {
		api     string
		reviews string
	}{
		"OVERRIDE_FROM_SELF_ERROR": {
			api: "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  name: String @override(from: \"petstore_api\")\n}\n",
		},
		"OVERRIDE_SOURCE_HAS_OVERRIDE": {
			api:     "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  name: String @override(from: \"petstore_reviews\")\n}\n",
			reviews: "type Pet @key(fields: \"id\") {\n  id: ID!\n  name: String @override(from: \"petstore_api\")\n}\n",
		},
		"TYPE_KIND_MISMATCH": {
			api:     "type Query {\n  pet: Pet\n}\n\ntype Pet {\n  name: String\n}\n",
			reviews: "interface Pet {\n  name: String\n}\n",
		},
		"FIELD_ARGUMENT_TYPE_MISMATCH": {
			api:     "type Query {\n  pets(limit: Int): [String] @shareable\n}\n",
			reviews: "type Query {\n  pets(limit: String): [String] @shareable\n}\n",
		},
		"FIELD_TYPE_MISMATCH": {
			api:     "type Query {\n  pets(filter: Filter): [String] @shareable\n}\n\ninput Filter {\n  status: Int\n}\n",
			reviews: "type Query {\n  pets(filter: Filter): [String] @shareable\n}\n\ninput Filter {\n  status: String\n}\n",
		},
		"EXTERNAL_MISSING_ON_BASE": {
			api: "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  name: String @external\n}\n",
		},
		"ONLY_INACCESSIBLE_CHILDREN": {
			api: "type Query {\n  audit: Audit\n}\n\ntype Audit {\n  entries: Int @inaccessible\n}\n",
		},
		"EMPTY_MERGED_INPUT_TYPE": {
			api:     "type Query {\n  pets(filter: Filter): [String] @shareable\n}\n\ninput Filter {\n  status: String\n}\n",
			reviews: "type Query {\n  pets(filter: Filter): [String] @shareable\n}\n\ninput Filter {\n  species: String\n}\n",
		},
		"EMPTY_MERGED_ENUM_TYPE": {
			api:     "type Query {\n  pets(status: Status): [String] @shareable\n}\n\nenum Status {\n  AVAILABLE\n}\n",
			reviews: "type Query {\n  pets(status: Status): [String] @shareable\n}\n\nenum Status {\n  SOLD\n}\n",
		},
		"REQUIRED_INACCESSIBLE": {
			api: "type Query {\n  pet(debug: Boolean! @inaccessible): String\n}\n",
		},
		"REQUIRED_ARGUMENT_MISSING_IN_SOME_SUBGRAPH": {
			api:     "type Query {\n  pets(limit: Int!): [String] @shareable\n}\n",
			reviews: "type Query {\n  pets: [String] @shareable\n}\n",
		},
		"REQUIRED_INPUT_FIELD_MISSING_IN_SOME_SUBGRAPH": {
			api:     "type Query {\n  pets(filter: Filter): [String] @shareable\n}\n\ninput Filter {\n  status: String\n  species: String!\n}\n",
			reviews: "type Query {\n  pets(filter: Filter): [String] @shareable\n}\n\ninput Filter {\n  status: String\n}\n",
		},
		"ENUM_VALUE_MISMATCH": {
			api:     "type Query {\n  pets(status: Status): Status @shareable\n}\n\nenum Status {\n  AVAILABLE\n  SOLD\n}\n",
			reviews: "type Query {\n  pets(status: Status): Status @shareable\n}\n\nenum Status {\n  AVAILABLE\n  PENDING\n}\n",
		},
		"REFERENCED_INACCESSIBLE": {
			api: "type Query {\n  pet: Pet\n}\n\ntype Pet @inaccessible {\n  name: String\n}\n",
		},
	}

	for code, tc := range cases {
		t.Run(code, func(t *testing.T) {
			members := []*model.PersistedContract{codesMember(t, "petstore_api", tc.api)}
			if tc.reviews != "" {
				members = append(members, codesMember(t, "petstore_reviews", tc.reviews))
			}

			_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", members)

			codes := make([]string, 0, len(errors))
			for _, composed := range errors {
				codes = append(codes, composed.Code)
				assert.NotContains(t, composed.Message, code)
			}

			assert.Contains(t, codes, code)
		})
	}
}
