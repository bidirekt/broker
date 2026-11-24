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

const satisfiabilityLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])
`

func satisfiabilityMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: satisfiabilityLink + sdl}})
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

func satisfiabilityErrors(subgraphs ...*model.PersistedContract) []string {
	_, errors := graphqlcontracts.ToComposedGraph("petstore_graph", subgraphs)

	return errors
}

const satisfiabilityAPISDL = `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const satisfiabilityReviewsSDL = `
type Pet @key(fields: "id"%s) {
  id: ID!
  reviews: [Review!]!
}

type Review {
  body: String!
}
`

func TestToComposedGraph_AFieldReachedThroughAResolvableKey_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", satisfiabilityAPISDL),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityReviewsSDL, "")),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AFieldBehindAKeyThatIsNotResolvable_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", satisfiabilityAPISDL),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityReviewsSDL, ", resolvable: false")),
	)

	assert.Equal(t, []string{"query pet.reviews can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet"}, errors)
}

const compositeKeyReviewsSDL = `
type Pet @key(fields: "name owner { id }") {
  name: String!
  owner: Owner!
  reviews: [Review!]!
}

type Owner {
  id: ID!
}

type Review {
  body: String!
}
`

func TestToComposedGraph_AFieldReachedThroughACompositeKey_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "name owner { id }") {
  name: String!
  owner: Owner!
}

type Owner {
  id: ID!
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", compositeKeyReviewsSDL),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_ACompositeKeyAFieldOfIsNotResolvedWhereTheQueryIs_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "name") {
  name: String!
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", compositeKeyReviewsSDL),
	)

	assert.Equal(t, []string{
		"query pet.owner can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet",
		"query pet.reviews can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet",
	}, errors)
}

func TestToComposedGraph_ATypeWithoutAPathFromQuery_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  latestReview: Review
}

type Review @shareable {
  id: ID!
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Review @shareable {
  id: ID!
  body: String!
}
`),
	)

	assert.Equal(t, []string{"query pet.latestReview.body can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Review"}, errors)
}

func TestToComposedGraph_ARecursiveType_IsWalkedOnce(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  parent: Pet
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityReviewsSDL, "")),
	)

	assert.Empty(t, errors)
}
