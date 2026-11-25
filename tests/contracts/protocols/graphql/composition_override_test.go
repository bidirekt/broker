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
)

const overrideLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.7", import: ["@key", "@shareable", "@override"])
`

func overrideMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: overrideLink + sdl}})
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

func overrideErrors(subgraphs ...*model.PersistedContract) []string {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", subgraphs)

	return errors
}

const overrideAPISDL = `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
  rating: Int
}
`

const overrideReviewsSDL = `
type Pet @key(fields: "id"%s) {
  id: ID!
  rating: Int%s
}
`

func TestToComposedGraph_AFieldTakenOverWithOverride_ComposesWithoutShareable(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		overrideMember(t, "petstore_api", "1.4.0", overrideAPISDL),
		overrideMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(overrideReviewsSDL, "", ` @override(from: "petstore_api")`)),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{
		"":       {Type: "object"},
		"id":     {Type: "string"},
		"name":   {Type: "string"},
		"rating": {Type: "integer", Nullable: true},
	}, composed.Schemas["graphql"]["Pet"])
}

func TestToComposedGraph_AFieldTakenOverWithOverride_IsResolvedOnlyByTheSubgraphThatTakesIt(t *testing.T) {
	errors := overrideErrors(
		overrideMember(t, "petstore_api", "1.4.0", overrideAPISDL),
		overrideMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(overrideReviewsSDL, ", resolvable: false", ` @override(from: "petstore_api")`)),
	)

	assert.Equal(t, []string{"query pet.rating can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet"}, errors)
}

func TestToComposedGraph_ARootFieldTakenOverWithOverride_StartsItsQueriesInTheSubgraphThatTakesIt(t *testing.T) {
	errors := overrideErrors(
		overrideMember(t, "petstore_api", "1.4.0", "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\", resolvable: false) {\n  id: ID!\n  name: String!\n}\n"),
		overrideMember(t, "petstore_reviews", "2.0.0", "type Query {\n  pet(id: ID!): Pet @override(from: \"petstore_api\")\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"),
	)

	assert.Equal(t, []string{"query pet.name can't be resolved: no resolvable @key leads from petstore_reviews 2.0.0 to petstore_api 1.4.0 on Pet"}, errors)
}

func TestToComposedGraph_AnOverrideFromItsOwnSubgraph_FailsTheComposition(t *testing.T) {
	errors := overrideErrors(
		overrideMember(t, "petstore_api", "1.4.0", "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"),
		overrideMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(overrideReviewsSDL, "", ` @override(from: "petstore_reviews")`)),
	)

	assert.Equal(t, []string{"petstore_reviews 2.0.0 overrides Pet.rating from itself"}, errors)
}

func TestToComposedGraph_AnOverrideFromASubgraphThatOverridesTheFieldToo_FailsTheComposition(t *testing.T) {
	errors := overrideErrors(
		overrideMember(t, "petstore_api", "1.4.0", "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  rating: Int @shareable @override(from: \"petstore_orders\")\n}\n"),
		overrideMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(overrideReviewsSDL, "", ` @shareable @override(from: "petstore_api")`)),
	)

	assert.Equal(t, []string{"petstore_reviews 2.0.0 overrides Pet.rating from petstore_api, which overrides it too"}, errors)
}

func TestToComposedGraph_AnOverrideFromASubgraphOutOfTheGraph_ChangesNothing(t *testing.T) {
	compose := func(directive string) []string {
		return overrideErrors(
			overrideMember(t, "petstore_api", "1.4.0", overrideAPISDL),
			overrideMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(overrideReviewsSDL, "", directive)),
		)
	}

	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_reviews 2.0.0 both resolve Pet.rating without @shareable"}, compose(""))
	assert.Equal(t, compose(""), compose(` @override(from: "petstore_legacy")`))
}

func TestToComposedGraph_AProgressiveOverride_TakesTheWholeField(t *testing.T) {
	errors := overrideErrors(
		overrideMember(t, "petstore_api", "1.4.0", overrideAPISDL),
		overrideMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(overrideReviewsSDL, ", resolvable: false", ` @override(from: "petstore_api", label: "percent(50)")`)),
	)

	assert.Equal(t, []string{"query pet.rating can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet"}, errors)
}
