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

const satisfiabilityLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@interfaceObject"])
`

var satisfiabilityGraph = protocol.Declarations{ExtensionPieces: map[string][]protocol.ExtensionPiece{
	"graphql": {{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"}}},
}}

func satisfiabilityMember(t *testing.T, name, version, sdl string) *model.PersistedContract {
	t.Helper()

	declarations, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{{Source: name + ".graphqls", Content: satisfiabilityLink + sdl}}, satisfiabilityGraph)
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

func TestToComposedGraph_AFieldOnlyAnotherSubgraphResolvesOnATypeWithoutAKey_CannotBeResolved(t *testing.T) {
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

func TestToComposedGraph_AQueryFromARootTheSchemaNamesOtherwise_IsWalkedLikeAnyQuery(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", "schema {\n  query: RootQuery\n}\n\ntype RootQuery {\n  pet(id: ID!): Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n  name: String!\n}\n"),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityReviewsSDL, ", resolvable: false")),
	)

	assert.Equal(t, []string{"query pet.reviews can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet"}, errors)
}

func TestToComposedGraph_ATypeNoQueryReaches_IsNotChecked(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", satisfiabilityAPISDL+`
type Owner @key(fields: "id") {
  id: ID!
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Owner @key(fields: "id", resolvable: false) {
  id: ID!
  email: String!
}
`),
	)

	assert.Empty(t, errors)
}

const providesLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@external", "@provides"])
`

func TestToComposedGraph_AnExternalFieldReachedThroughAProvides_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  reviews: [Review!]!
}

type Review {
  id: ID!
  author: Owner @provides(fields: "name")
}

type Owner @key(fields: "id") {
  id: ID!
  name: String @external
}
`),
		satisfiabilityMember(t, "petstore_owners", "1.4.0", `
type Owner @key(fields: "id", resolvable: false) {
  id: ID!
  name: String @shareable
}
`),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AnExternalFieldReachedWithoutTheProvides_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  reviews: [Review!]!
  owner: Owner
}

type Review {
  id: ID!
  author: Owner @provides(fields: "name")
}

type Owner @key(fields: "id") {
  id: ID!
  name: String @external
}
`),
		satisfiabilityMember(t, "petstore_owners", "1.4.0", `
type Owner @key(fields: "id", resolvable: false) {
  id: ID!
  name: String @shareable
}
`),
	)

	assert.Equal(t, []string{"query owner.name can't be resolved: no resolvable @key leads from petstore_reviews 2.0.0 to petstore_owners 1.4.0 on Owner"}, errors)
}

func TestToComposedGraph_ATypeExternalExtensionReachedThroughAProvides_LeavesOnlyTheKeyFieldUnresolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  reviews: [Review!]!
}

type Review {
  id: ID!
  author: Owner @provides(fields: "name email")
}

type Owner @key(fields: "id") {
  id: ID!
}

extend type Owner @external {
  name: String
  email: String
}
`),
		satisfiabilityMember(t, "petstore_owners", "1.4.0", `
type Owner @key(fields: "id") {
  id: ID!
  name: String @shareable
  email: String @shareable
}
`),
	)

	assert.Equal(t, []string{"query reviews.author.id can't be resolved: no resolvable @key leads from petstore_reviews 2.0.0 to petstore_owners 1.4.0 on Owner"}, errors)
}

func TestToComposedGraph_AFieldANestedProvidesSelects_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  reviews: [Review!]!
}

type Review {
  id: ID!
  author: Owner @provides(fields: "address { city }")
}

type Owner @key(fields: "id") {
  id: ID!
  address: Address @external
}

type Address {
  city: String @external
}
`),
		satisfiabilityMember(t, "petstore_owners", "1.4.0", `
type Owner @key(fields: "id", resolvable: false) {
  id: ID!
  address: Address @shareable
}

type Address {
  city: String @shareable
}
`),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AFieldAProvidesSelectsThroughAnInlineFragment_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  items: [Item!]! @provides(fields: "... on Book { title }")
}

interface Item {
  id: ID!
}

type Book implements Item @key(fields: "id") {
  id: ID!
  title: String @external
}
`),
		satisfiabilityMember(t, "petstore_catalog", "1.4.0", `
interface Item {
  id: ID!
}

type Book implements Item @key(fields: "id", resolvable: false) {
  id: ID!
  title: String @shareable
}
`),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AKeyAProvidesSelects_LeadsToTheSubgraphThatResolvesTheRest(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  reviews: [Review!]!
}

type Review {
  id: ID!
  author: Owner @provides(fields: "id")
}

type Owner @key(fields: "id") {
  id: ID! @external
}
`),
		satisfiabilityMember(t, "petstore_owners", "1.4.0", `
type Owner @key(fields: "id") {
  id: ID!
  name: String
}
`),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AFieldTheProvidesOfARootFieldSelects_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", providesLink+`
type Query {
  me: Owner @provides(fields: "name")
}

type Owner @key(fields: "id") {
  id: ID!
  name: String @external
}
`),
		satisfiabilityMember(t, "petstore_owners", "1.4.0", `
type Owner @key(fields: "id", resolvable: false) {
  id: ID!
  name: String @shareable
}
`),
	)

	assert.Empty(t, errors)
}

const satisfiabilityUnionHolderSDL = `
type Query {
  %s: Holder @shareable
}

type Holder @shareable {
  item: Result
}

union Result = Book

type Book @shareable {
  id: ID!
}
`

const satisfiabilityBookHolderSDL = `
type Query {
  %s: Holder @shareable
}

type Holder @shareable {
  item: Book
}

type Book @shareable {
  id: ID!
  title: String
}
`

func TestToComposedGraph_AFieldOfTheMemberASubgraphReturnsForTheUnion_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", fmt.Sprintf(satisfiabilityUnionHolderSDL, "holder")),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityBookHolderSDL, "holder")),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AFieldOfTheMemberReachedOnlyFromTheSubgraphReturningTheUnion_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", fmt.Sprintf(satisfiabilityUnionHolderSDL, "api")),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityBookHolderSDL, "reviews")),
	)

	assert.Equal(t, []string{"query api.item.title can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Book"}, errors)
}

func TestToComposedGraph_AFieldOfAMemberASubgraphAtTheUnionNeverReturns_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  holder: Holder @shareable
}

type Holder @shareable {
  item: Result
}

union Result = Book | Movie

type Book @shareable {
  id: ID!
}

type Movie @shareable {
  name: String
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Query {
  holder: Holder @shareable
}

type Holder @shareable {
  item: Book
}

type Book @shareable {
  id: ID!
}

type Movie @shareable {
  name: String
  year: Int
}
`),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AFieldOfAnImplementationASubgraphMarkingTheInterfaceObjectNeverReturns_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  media: Media @shareable
}

interface Media @key(fields: "id") {
  id: ID!
}

type Book implements Media @key(fields: "id") {
  id: ID!
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Query {
  media: Media @shareable
}

type Media @key(fields: "id") @interfaceObject {
  id: ID!
  reviews: [String!]!
}
`),
		satisfiabilityMember(t, "petstore_catalog", "3.0.0", `
type Book @key(fields: "id", resolvable: false) {
  id: ID!
  pages: Int
}
`),
	)

	assert.Equal(t, []string{"query media.pages can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_catalog 3.0.0 on Book"}, errors)
}

func TestToComposedGraph_AFieldOfTheImplementationASubgraphReturnsForTheInterface_Composes(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  holder: Holder @shareable
}

type Holder @shareable {
  item: Node
}

interface Node {
  id: ID!
}

type Thing implements Node @shareable {
  id: ID!
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Query {
  holder: Holder @shareable
}

type Holder @shareable {
  item: Thing
}

type Thing @shareable {
  id: ID!
  rating: Int
}
`),
	)

	assert.Empty(t, errors)
}

const satisfiabilityNodeHolderSDL = `
type Query {
  %s: Holder @shareable
}

type Holder @shareable {
  item: Node
}

interface Node {
  id: ID!
}

type Dog implements Node @shareable {
  id: ID!
}
`

const satisfiabilityAnimalHolderSDL = `
type Query {
  %s: Holder @shareable
}

type Holder @shareable {
  item: Animal
}

interface Node {
  id: ID!
}

interface Animal implements Node {
  id: ID!
  name: String
}

type Dog implements Animal & Node @shareable {
  id: ID!
  name: String
  color: String
}
`

func TestToComposedGraph_FieldsOfAnImplementationASubgraphReturnsForAnInterfaceThatImplementsTheOther_Compose(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", fmt.Sprintf(satisfiabilityNodeHolderSDL, "holder")),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityAnimalHolderSDL, "holder")),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_FieldsOfAnImplementationReachedOnlyFromTheSubgraphReturningTheWiderInterface_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", fmt.Sprintf(satisfiabilityNodeHolderSDL, "api")),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", fmt.Sprintf(satisfiabilityAnimalHolderSDL, "reviews")),
	)

	assert.Equal(t, []string{
		"query api.item.color can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Dog",
		"query api.item.name can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Dog",
	}, errors)
}

func TestToComposedGraph_AQueryRootAPayloadReturns_EntersEverySubgraphThatDefinesIt(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  status: String
}

type Mutation {
  addPet(name: String!): AddPetPayload
}

type AddPetPayload {
  query: Query
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Query {
  reviews: [Review!]!
}

type Review {
  body: String!
}
`),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_AMutationRootAMutationReturns_EntersEverySubgraphThatDefinesIt(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", `
type Query {
  status: String
}

type Mutation {
  chain: Mutation
}
`),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", `
type Query {
  reviews: [Review!]!
}

type Mutation {
  addReview(body: String!): Review
}

type Review {
  body: String!
}
`),
	)

	assert.Empty(t, errors)
}

const nestedKeyAPISDL = `
type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
  owner: Owner @shareable%s
}

type Owner @key(fields: "id") {
  id: ID!
  email: String @external
}
`

const nestedKeyReviewsSDL = `
type Pet @key(fields: "owner { email }") {
  owner: Owner @shareable
  reviews: [Review!]!
}

type Owner @key(fields: "id") {
  id: ID!
  email: String @shareable
}

type Review {
  body: String!
}
`

func TestToComposedGraph_ANestedKeyFieldTheProvidesOfItsParentFieldSelects_LeadsToTheSubgraphThatResolvesTheRest(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", providesLink+fmt.Sprintf(nestedKeyAPISDL, ` @provides(fields: "email")`)),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", nestedKeyReviewsSDL),
	)

	assert.Empty(t, errors)
}

func TestToComposedGraph_ANestedKeyFieldThatIsExternalWithoutAProvides_CannotBeResolved(t *testing.T) {
	errors := satisfiabilityErrors(
		satisfiabilityMember(t, "petstore_api", "1.4.0", providesLink+fmt.Sprintf(nestedKeyAPISDL, "")),
		satisfiabilityMember(t, "petstore_reviews", "2.0.0", nestedKeyReviewsSDL),
	)

	assert.Equal(t, []string{"query pet.reviews can't be resolved: no resolvable @key leads from petstore_api 1.4.0 to petstore_reviews 2.0.0 on Pet"}, errors)
}
