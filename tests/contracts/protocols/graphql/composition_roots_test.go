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

const rootsLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])
`

var rootsGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func rootsMember(t *testing.T, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: "api.graphqls", Content: rootsLink + sdl}}, rootsGraph)
	require.NoError(t, readErr)

	catalogued := protocol.Declarations{Resources: declarations.Resources}
	for _, schema := range declarations.Schemas {
		catalogued.AddSchema(schema)
	}

	extension, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	return &model.PersistedContract{
		ParticipantName: "petstore_api",
		Version:         "1.4.0",
		Schemas:         fragmentmapper.ToSchemaModels(catalogued),
		Extensions:      map[string]any{"graphql": extension},
	}
}

func rootsErrors(t *testing.T, sdl string) []string {
	t.Helper()

	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{rootsMember(t, sdl)})

	return errors
}

func rootsIdentities(t *testing.T, sdl string) []graphqlmodel.Identity {
	t.Helper()

	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{rootsMember(t, sdl)})
	require.Empty(t, errors)

	var identities []graphqlmodel.Identity
	for _, resource := range composed.Resources {
		identities = append(identities, resource.Identity.(graphqlmodel.Identity))
	}

	return identities
}

func TestToComposedGraph_AQueryRootNamedOtherwiseNextToATypeNamedQuery_FailsAsApolloDoes(t *testing.T) {
	errors := rootsErrors(t, "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Int\n}\n\ntype Query {\n  legacy: Int\n}\n")

	assert.Equal(t, []string{"petstore_api 1.4.0 has a type named Query, but its query root is RootQuery (ROOT_QUERY_USED)"}, errors)
}

func TestToComposedGraph_AQueryRootNamedOtherwiseWhileQueryIsTheMutationRoot_FailsAsApolloDoes(t *testing.T) {
	errors := rootsErrors(t, "schema {\n  query: RootQuery\n  mutation: Query\n}\n\ntype RootQuery {\n  pet: Int\n}\n\ntype Query {\n  adopt: Int\n}\n")

	assert.Equal(t, []string{"petstore_api 1.4.0 has a type named Query, but its query root is RootQuery (ROOT_QUERY_USED)"}, errors)
}

func TestToComposedGraph_AMutationRootNamedOtherwiseNextToATypeNamedMutation_FailsAsApolloDoes(t *testing.T) {
	errors := rootsErrors(t, "schema {\n  query: Query\n  mutation: RootMutation\n}\n\ntype Query {\n  pet: Int\n}\n\ntype RootMutation {\n  adopt: Int\n}\n\ntype Mutation {\n  legacy: Int\n}\n")

	assert.Equal(t, []string{"petstore_api 1.4.0 has a type named Mutation, but its mutation root is RootMutation (ROOT_MUTATION_USED)"}, errors)
}

func TestToComposedGraph_APlainTypeMutationNextToAQueryRootNamedOtherwise_IsNoMutation(t *testing.T) {
	identities := rootsIdentities(t, "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet: Int\n}\n\ntype Mutation {\n  notAMutation: Int\n}\n")

	assert.ElementsMatch(t, []graphqlmodel.Identity{
		{Operation: "query", Field: "pet", Interaction: "arguments"},
		{Operation: "query", Field: "pet", Interaction: "returns"},
	}, identities)
}

func TestToComposedGraph_APlainTypeMutationNextToADeclaredQueryRoot_IsNoMutation(t *testing.T) {
	identities := rootsIdentities(t, "schema {\n  query: Query\n}\n\ntype Query {\n  pet: Int\n}\n\ntype Mutation {\n  notAMutation: Int\n}\n")

	assert.ElementsMatch(t, []graphqlmodel.Identity{
		{Operation: "query", Field: "pet", Interaction: "arguments"},
		{Operation: "query", Field: "pet", Interaction: "returns"},
	}, identities)
}

func TestToComposedGraph_ATypeThatIsBothRoots_StaysTheQueryRoot(t *testing.T) {
	identities := rootsIdentities(t, "schema {\n  query: Query\n  mutation: Query\n}\n\ntype Query {\n  pet: Int\n}\n")

	assert.ElementsMatch(t, []graphqlmodel.Identity{
		{Operation: "query", Field: "pet", Interaction: "arguments"},
		{Operation: "query", Field: "pet", Interaction: "returns"},
	}, identities)
}
