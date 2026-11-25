package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	declarations, err := graphqlcontracts.NewSDLFormat().ToDeclarations(files)
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
