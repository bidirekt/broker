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

const directivesLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@external", "@requires", "@tag"])
`

const directivesSDL = directivesLink + `
type Query {
  pet(id: ID! @inaccessible): Pet
}

type Pet @key(fields: "id") @key(fields: "name  ,  owner{id}") {
  id: ID!
  name: String! @shareable
  owner: Owner!
  weight: Int @external
  shippingCost: Int @requires(fields: "weight")
  nickname: String @tag(name: "public")
  status: Status
}

type Owner @key(fields: "id", resolvable: false) @shareable {
  id: ID!
}

enum Status {
  AVAILABLE @inaccessible
  SOLD
}
`

func mergedDirectives(t *testing.T, files ...model.ContractFile) graphqlmodel.Extension {
	t.Helper()

	declarations, err := graphqlcontracts.NewSDLFormat().ToDeclarations(files, protocol.Declarations{})
	require.NoError(t, err)

	merged, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	return merged.(graphqlmodel.Extension)
}

func TestSDLFormat_Federation_TheDirectivesThatCount_AreKeptSortedWithANormalizedFieldSet(t *testing.T) {
	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: directivesSDL})

	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Owner", Directive: "key", Fields: "id", Resolvable: false},
		{Coordinate: "Owner.id", Directive: "shareable", Resolvable: true},
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet", Directive: "key", Fields: "name owner { id }", Resolvable: true},
		{Coordinate: "Pet.name", Directive: "shareable", Resolvable: true},
		{Coordinate: "Pet.weight", Directive: "external", Resolvable: true},
		{Coordinate: "Query.pet(id:)", Directive: "inaccessible", Resolvable: true},
		{Coordinate: "Status.AVAILABLE", Directive: "inaccessible", Resolvable: true},
	}, merged.Directives)
}

func TestSDLFormat_Federation_DirectivesInAnotherOrderAndSpacing_MergeTheSame(t *testing.T) {
	reordered := directivesLink + `
type Query {
  pet(id: ID! @inaccessible): Pet
}

enum Status {
  AVAILABLE @inaccessible
  SOLD
}

type Owner @shareable @key(resolvable: false, fields: "id") {
  id: ID!
}

type Pet @key(fields: "name owner { id }") @key(fields: " id ") {
  id: ID!
  name: String! @shareable
  owner: Owner!
  weight: Int @external
  shippingCost: Int @requires(fields: "weight")
  nickname: String @tag(name: "public")
  status: Status
}
`

	assert.Equal(t,
		mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: directivesSDL}),
		mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: reordered}),
	)
}

func TestSDLFormat_Federation_TheSameSDLSplitIntoOtherFiles_MergesTheSameDirectives(t *testing.T) {
	whole := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: directivesSDL})
	split := mergedDirectives(t,
		model.ContractFile{Source: "a.graphqls", Content: directivesLink},
		model.ContractFile{Source: "b.graphqls", Content: directivesSDL[len(directivesLink):]},
	)

	assert.Equal(t, whole.Directives, split.Directives)
	assert.Equal(t, whole.Types, split.Types)
}

func TestSDLFormat_Federation_ARenamedImportAndTheNamespace_CountByTheirFederationName(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", as: "fed", import: [{name: "@key", as: "@primaryKey"}])

type Query {
  pet: Pet
}

type Pet @primaryKey(fields: "id") {
  id: ID!
  name: String! @fed__shareable
}
`

	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet.name", Directive: "shareable", Resolvable: true},
	}, merged.Directives)
}

func TestSDLFormat_Federation_TheShareableOfAType_SharesOnlyTheFieldsOfItsOwnDeclaration(t *testing.T) {
	sdl := directivesLink + `
type Query {
  price: Money
}

type Money @shareable {
  amount: Int
}

extend type Money {
  currency: String
}
`

	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []graphqlmodel.Directive{{Coordinate: "Money.amount", Directive: "shareable", Resolvable: true}}, merged.Directives)
}

func TestSDLFormat_Federation_AShareableTypeAndAShareableExtensionOfIt_ShareEachTheirOwnFields(t *testing.T) {
	sdl := directivesLink + `
type Query {
  owner: Owner
}

type Owner @shareable {
  id: ID!
}

extend type Owner @shareable {
  name: String
}
`

	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Owner.id", Directive: "shareable", Resolvable: true},
		{Coordinate: "Owner.name", Directive: "shareable", Resolvable: true},
	}, merged.Directives)
}

func TestSDLFormat_Federation_AShareableFieldOfAShareableType_IsKeptOnce(t *testing.T) {
	sdl := directivesLink + `
type Query {
  owner: Owner
}

type Owner @shareable {
  id: ID! @shareable
}
`

	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []graphqlmodel.Directive{{Coordinate: "Owner.id", Directive: "shareable", Resolvable: true}}, merged.Directives)
}

func TestSDLFormat_Federation_ARenamedShareableOnAType_SharesOnlyTheFieldsOfItsOwnDeclaration(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", as: "fed", import: [{name: "@shareable", as: "@shared"}])

type Query {
  price: Money
}

type Money @shared {
  amount: Int
}

extend type Money {
  currency: String
}
`

	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []graphqlmodel.Directive{{Coordinate: "Money.amount", Directive: "shareable", Resolvable: true}}, merged.Directives)
}

func TestSDLFormat_Federation_TheFileOfTheLink_IsTheLinkSource(t *testing.T) {
	merged := mergedDirectives(t,
		model.ContractFile{Source: "b.graphqls", Content: directivesLink},
		model.ContractFile{Source: "a.graphqls", Content: directivesSDL[len(directivesLink):]},
	)

	assert.Equal(t, "b.graphqls", merged.LinkSource)
}

func TestSDLFormat_WithoutALink_KeepsNoDirectiveAndNoLinkSource(t *testing.T) {
	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: "type Query {\n  pet: Pet\n}\n\ntype Pet {\n  id: ID!\n}\n"})

	assert.Empty(t, merged.Directives)
	assert.Empty(t, merged.LinkSource)
}

func TestSDLFormat_Federation_AnSDLThatDeclaresTheFederationDefinitions_KeepsTheSameDirectives(t *testing.T) {
	declared := directivesLink + `
directive @link(url: String!, as: String, for: link__Purpose, import: [link__Import]) repeatable on SCHEMA

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

scalar link__Import

enum link__Purpose {
  SECURITY
  EXECUTION
}

scalar FieldSet
` + directivesSDL[len(directivesLink):]

	assert.Equal(t,
		mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: directivesSDL}).Directives,
		mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: declared}).Directives,
	)
}

func TestSDLFormat_Federation_AKeyRepeatedWithAndWithoutResolvable_IsKeptResolvableInEitherOrder(t *testing.T) {
	withResolvable := func(keys string) []graphqlmodel.Directive {
		return mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: directivesLink + `
type Query {
  owner: Owner
}

type Owner ` + keys + ` {
  id: ID!
}
`}).Directives
	}

	expected := []graphqlmodel.Directive{{Coordinate: "Owner", Directive: "key", Fields: "id", Resolvable: true}}
	assert.Equal(t, expected, withResolvable(`@key(fields: "id", resolvable: false) @key(fields: "id")`))
	assert.Equal(t, expected, withResolvable(`@key(fields: "id") @key(fields: "id", resolvable: false)`))
}

func TestSDLFormat_Federation_AnOverride_IsKeptWithTheSubgraphItTakesTheFieldFrom(t *testing.T) {
	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@override"])

type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
  rating: Int @override(from: "petstore_api")
}
`})

	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet.rating", Directive: "override", From: "petstore_api", Resolvable: true},
	}, merged.Directives)
}

func TestSDLFormat_Federation_AProvides_IsKeptWithANormalizedFieldSet(t *testing.T) {
	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@external", "@provides"])

type Query {
  me: User @provides(fields: "name")
  items: [Item!]! @provides(fields: "...on Book{title}")
  reviews: [Review!]!
}

type Review {
  id: ID!
  author: User @provides(fields: " name,address{city} ")
}

type User @key(fields: "id") {
  id: ID!
  name: String @external
  address: Address @external
}

type Address {
  city: String @external
}

interface Item {
  id: ID!
}

type Book implements Item @key(fields: "id") {
  id: ID!
  title: String @external
}
`})

	assert.Equal(t, []graphqlmodel.Directive{
		{Coordinate: "Address.city", Directive: "external", Resolvable: true},
		{Coordinate: "Book", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Book.title", Directive: "external", Resolvable: true},
		{Coordinate: "Query.items", Directive: "provides", Fields: "... on Book { title }", Resolvable: true},
		{Coordinate: "Query.me", Directive: "provides", Fields: "name", Resolvable: true},
		{Coordinate: "Review.author", Directive: "provides", Fields: "name address { city }", Resolvable: true},
		{Coordinate: "User", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "User.address", Directive: "external", Resolvable: true},
		{Coordinate: "User.name", Directive: "external", Resolvable: true},
	}, merged.Directives)
}

func TestSDLFormat_Federation_TheLabelOfAnOverride_IsReadAndDropped(t *testing.T) {
	overriding := func(arguments string) []graphqlmodel.Directive {
		return mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: `extend schema @link(url: "https://specs.apollo.dev/federation/v2.7", import: ["@override"])

type Query {
  rating: Int @override(` + arguments + `)
}
`}).Directives
	}

	assert.Equal(t,
		[]graphqlmodel.Directive{{Coordinate: "Query.rating", Directive: "override", From: "petstore_api", Resolvable: true}},
		overriding(`from: "petstore_api", label: "percent(50)"`),
	)
}

const externalsLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@external", "@extends", "@requires", "@shareable"])
`

func externalCoordinates(t *testing.T, sdl string) []string {
	t.Helper()

	var coordinates []string
	for _, directive := range mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl}).Directives {
		if directive.Directive == "external" {
			coordinates = append(coordinates, directive.Coordinate)
		}
	}

	return coordinates
}

func TestSDLFormat_Federation_AnExternalFieldTheKeyOfAnExtendTypeSelects_IsNotExternal(t *testing.T) {
	sdl := externalsLink + `
type Query {
  topReviews: [Review!]!
}

type Review {
  body: String
  pet: Pet
}

type Pet {
  name: String
}

extend type Pet @key(fields: "id owner { id }") {
  id: ID! @external
  owner: Owner @external
  weight: Int @external
  reviews: [Review!]!
}

type Owner {
  id: ID! @external
}
`

	assert.Equal(t, []string{"Pet.weight"}, externalCoordinates(t, sdl))
}

func TestSDLFormat_Federation_AnExternalFieldTheKeyOfATypeMarkedExtendsSelects_IsNotExternal(t *testing.T) {
	sdl := externalsLink + `
type Query {
  topReviews: [Review!]!
}

type Review {
  pet: Pet
}

type Pet @extends @key(fields: "id") {
  id: ID! @external
  weight: Int @external
  reviews: [Review!]!
}
`

	assert.Equal(t, []string{"Pet.weight"}, externalCoordinates(t, sdl))
}

func TestSDLFormat_Federation_AnExternalFieldTheKeyOfTheBaseTypeSelects_StaysExternal(t *testing.T) {
	sdl := externalsLink + `
type Query {
  topReviews: [Review!]!
}

type Review {
  pet: Pet
}

type Pet @key(fields: "id") {
  reviews: [Review!]!
}

extend type Pet {
  id: ID! @external
}
`

	assert.Equal(t, []string{"Pet.id"}, externalCoordinates(t, sdl))
}

func TestSDLFormat_Federation_AnExternalType_MakesEveryFieldExternal_ItsExtensionsIncluded(t *testing.T) {
	sdl := externalsLink + `
type Query {
  pet: Pet
}

type Pet @key(fields: "id") {
  id: ID!
  dimensions: Dimensions @external
  shippingCost: Int @requires(fields: "dimensions { height }")
}

type Dimensions @external {
  height: Int
  width: Int
}

extend type Dimensions {
  depth: Int
}
`

	merged := mergedDirectives(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []string{"Dimensions.depth", "Dimensions.height", "Dimensions.width", "Pet.dimensions"}, externalCoordinates(t, sdl))
	assert.NotContains(t, merged.Directives, graphqlmodel.Directive{Coordinate: "Dimensions", Directive: "external", Resolvable: true})
}

func TestSDLFormat_Federation_AnExternalTypeTheKeyOfAnExtendTypeSelects_StaysExternal(t *testing.T) {
	sdl := externalsLink + `
type Query {
  pet: Pet
}

type Pet @external {
  name: String
}

extend type Pet @key(fields: "id") {
  id: ID!
}
`

	assert.Equal(t, []string{"Pet.id", "Pet.name"}, externalCoordinates(t, sdl))
}
