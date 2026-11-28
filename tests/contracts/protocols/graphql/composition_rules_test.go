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

const rulesLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@external"])
`

var rulesGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func rulesMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: rulesLink + sdl}}, rulesGraph)
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

func rulesErrors(subgraphs ...*model.PersistedContract) []string {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", subgraphs)

	return graphqlcontracts.CompositionMessages(errors)
}

const rulesArgumentSDL = `
type Query {
  pet(id: ID!, debug: %s @inaccessible): Pet
}

type Pet {
  id: ID!
}
`

func TestToComposedGraph_ARequiredArgumentThatIsInaccessible_Fails(t *testing.T) {
	errors := rulesErrors(rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesArgumentSDL, "Boolean!")))

	assert.Equal(t, []string{"petstore_api 1.4.0 requires Query.pet(debug:), which is @inaccessible"}, errors)
}

func TestToComposedGraph_AnOptionalArgumentThatIsInaccessible_Composes(t *testing.T) {
	assert.Empty(t, rulesErrors(rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesArgumentSDL, "Boolean"))))
}

const rulesInputFieldSDL = `
type Query {
  pets(filter: PetFilter): [Pet!]!
}

input PetFilter {
  name: String
  secret: %s @inaccessible
}

type Pet {
  id: ID!
}
`

func TestToComposedGraph_ARequiredInputFieldThatIsInaccessible_Fails(t *testing.T) {
	errors := rulesErrors(rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesInputFieldSDL, "String!")))

	assert.Equal(t, []string{"petstore_api 1.4.0 requires PetFilter.secret, which is @inaccessible"}, errors)
}

func TestToComposedGraph_AnOptionalInputFieldThatIsInaccessible_Composes(t *testing.T) {
	assert.Empty(t, rulesErrors(rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesInputFieldSDL, "String"))))
}

const rulesReferenceSDL = `
type Query {
  pet: Pet
  pets(filter: PetFilter): [Pet!]!
}

type Pet {
  id: ID!
  audit: Audit%s
}

type Audit @inaccessible {
  entries: Int
}

input PetFilter %s {
  name: String
}
`

func TestToComposedGraph_AVisibleFieldOrArgumentWhoseTypeIsInaccessible_Fails(t *testing.T) {
	errors := rulesErrors(rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesReferenceSDL, "", "@inaccessible")))

	assert.Equal(t, []string{
		"petstore_api 1.4.0 declares Pet.audit as Audit, which is @inaccessible",
		"petstore_api 1.4.0 declares Query.pets(filter:) as PetFilter, which is @inaccessible",
	}, errors)
}

func TestToComposedGraph_AnInaccessibleTypeOnlyInaccessibleElementsUse_Composes(t *testing.T) {
	assert.Empty(t, rulesErrors(rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesReferenceSDL, " @inaccessible", ""))))
}

func TestToComposedGraph_AnObjectWhoseFieldsAreAllInaccessible_Fails(t *testing.T) {
	errors := rulesErrors(rulesMember(t, "petstore_api", "1.4.0", "type Query {\n  audit: Audit\n}\n\ntype Audit {\n  entries: Int @inaccessible\n}\n"))

	assert.Equal(t, []string{"Audit has no field left in the graph of petstore_api 1.4.0"}, errors)
}

func TestToComposedGraph_AnInputTheSubgraphsShareNoFieldOf_Fails(t *testing.T) {
	errors := rulesErrors(
		rulesMember(t, "petstore_api", "1.4.0", "type Query {\n  pets(filter: PetFilter): Int @shareable\n}\n\ninput PetFilter {\n  name: String\n}\n"),
		rulesMember(t, "petstore_reviews", "2.0.0", "type Query {\n  pets(filter: PetFilter): Int @shareable\n}\n\ninput PetFilter {\n  rating: Int\n}\n"),
	)

	assert.Equal(t, []string{"PetFilter has no field left in the graph of petstore_api 1.4.0 and petstore_reviews 2.0.0"}, errors)
}

const rulesEnumSDL = `
type Query {
  pets(size: Size): Int @shareable
}

enum Size {
  %s
}
`

func TestToComposedGraph_AnInputEnumTheSubgraphsShareNoValueOf_Fails(t *testing.T) {
	errors := rulesErrors(
		rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesEnumSDL, "SMALL")),
		rulesMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(rulesEnumSDL, "LARGE")),
	)

	assert.Equal(t, []string{"Size has no value left in the graph of petstore_api 1.4.0 and petstore_reviews 2.0.0"}, errors)
}

func TestToComposedGraph_AnInputEnumTheSubgraphsShareAValueOf_Composes(t *testing.T) {
	assert.Empty(t, rulesErrors(
		rulesMember(t, "petstore_api", "1.4.0", fmt.Sprintf(rulesEnumSDL, "SMALL\n  LARGE")),
		rulesMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(rulesEnumSDL, "LARGE")),
	))
}

const rulesExternalSDL = `
type Pet @key(fields: "id") {
  id: ID!
  weight: Int @external%s
}
`

func TestToComposedGraph_AFieldOnlyExternalWhereverItIsDeclared_Fails(t *testing.T) {
	errors := rulesErrors(
		rulesMember(t, "petstore_api", "1.4.0", "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"),
		rulesMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(rulesExternalSDL, "")),
	)

	assert.Equal(t, []string{"petstore_reviews 2.0.0 marks Pet.weight @external, but no subgraph resolves it"}, errors)
}

func TestToComposedGraph_AFieldOnlyExternalThatIsInaccessible_LeavesTheGraph(t *testing.T) {
	composed, errors := graphqlcontracts.ToComposedGraph("petstore_graph", []*model.PersistedContract{
		rulesMember(t, "petstore_api", "1.4.0", "type Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"),
		rulesMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(rulesExternalSDL, " @inaccessible")),
	})
	require.Empty(t, errors)

	assert.Equal(t, model.Schema{"": {Type: "object"}, "id": {Type: "string", Scalar: "ID"}}, composed.Schemas["graphql"]["Pet"])
}
