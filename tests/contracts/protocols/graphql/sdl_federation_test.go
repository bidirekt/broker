package graphql_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const federatedProductsSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@external", "@requires", "@tag", "FieldSet"])

type Query {
  product(upc: String!): Product
}

type Product @key(fields: "upc") @key(fields: "sku  ,  variant { id }") {
  upc: String!
  sku: String! @shareable
  variant: Variant
  internalCode: String @inaccessible
  weight: Int @external
  shippingEstimate: Int @requires(fields: "weight")
  name: String @tag(name: "public")
}

type Variant @key(fields: "id", resolvable: false) {
  id: ID!
}
`

func readFederatedSDL(files ...model.ContractFile) (protocol.Declarations, error) {
	return graphqlcontracts.NewSDLFormat().ToDeclarations(files, protocol.Declarations{})
}

func mergedFederationExtension(t *testing.T, files ...model.ContractFile) graphqlmodel.Extension {
	t.Helper()

	declarations, err := readFederatedSDL(files...)
	require.NoError(t, err)

	merged, violations := graphqlcontracts.New().MergeExtensions(declarations.ExtensionPieces["graphql"])
	require.Empty(t, violations)

	return merged.(graphqlmodel.Extension)
}

func federatedSchemaNames(t *testing.T, sdl string) []string {
	t.Helper()

	declarations, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})
	require.NoError(t, err)

	return federatedSchemaNamesOf(declarations)
}

func federatedSchemaNamesOf(declarations protocol.Declarations) []string {
	names := make([]string, 0, len(declarations.Schemas))
	for _, declaration := range declarations.Schemas {
		names = append(names, declaration.Name)
	}

	return names
}

func TestSDLFormat_Federation_ReadsTheDirectivesItImports(t *testing.T) {
	names := federatedSchemaNames(t, federatedProductsSDL)

	assert.ElementsMatch(t, []string{"Product", "Query", "Variant", "Query:product:arguments", "Query:product:returns"}, names)
}

func TestSDLFormat_Federation_EveryFederation2VersionIsRead_KnownOrNot(t *testing.T) {
	for minor := 0; minor <= 16; minor++ {
		version := fmt.Sprintf("v2.%d", minor)
		sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/` + version + `", import: ["@key", "@shareable"])

type Query {
  product: Product
}

type Product @key(fields: "upc") {
  upc: String! @shareable
}
`
		_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

		assert.NoError(t, err, version)
	}
}

func TestSDLFormat_Federation_EveryDirectiveOfTheDefinitionSet_IsUsedThroughTheNamespace(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.0")
  @federation__composeDirective(name: "@custom")
  @federation__tag(name: "schema")

directive @custom on OBJECT

type Query {
  pet(
    id: ID! @federation__cost(weight: 1) @federation__inaccessible @federation__tag(name: "argument")
    owner: String @federation__fromContext(field: "$petContext { owner }")
  ): Pet @federation__listSize(assumedSize: 10, slicingArguments: ["first"], sizedFields: ["edges"], requireOneSlicingArgument: false)
}

type Pet @federation__key(fields: "id", resolvable: false) @federation__context(name: "petContext") @federation__context(name: "animalContext")
  @federation__authenticated @federation__requiresScopes(scopes: [["read"]]) @federation__policy(policies: [["owner"]])
  @federation__cacheTag(format: "pet-{$key.id}") @federation__cacheTag(format: "pets") @federation__shareable @federation__shareable {
  id: ID!
  owner: String @federation__external(reason: "owned elsewhere")
  weight: Int @federation__requires(fields: "owner")
  rating: Int @federation__override(from: "petstore_api", label: "percent(50)")
  friend: Pet @federation__provides(fields: "id")
}

type Dimensions @federation__external(reason: "measured elsewhere") @federation__interfaceObject @federation__extends {
  height: Int
}
`
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.NoError(t, err)
}

func TestSDLFormat_Federation_ADirectiveTheDefinitionSetLacks_IsUndefinedThroughTheNamespace(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.15\")\n\ntype Query {\n  pet: Int @federation__unknown\n}\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:4:13: Undefined directive federation__unknown.`, err.Error())
}

func TestSDLFormat_Federation_TheDirectivesApolloDefinesBeyondTheEmbeddedVersions_ArePublished(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.12", import: ["@key", "@external", "@tag", "@context", "@cacheTag"])
  @tag(name: "public")

type Query {
  pet: Pet
}

type Pet @key(fields: "id") @context(name: "petContext") @context(name: "animalContext") @cacheTag(format: "pet-{$key.id}") {
  id: ID!
  weight: Int @external(reason: "the scale service owns it")
}
`
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.NoError(t, err)
}

func TestSDLFormat_Federation_ANotImportedDirectiveIsUsedThroughTheNamespace(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Product @key(fields: "upc") {
  upc: String! @federation__shareable
}
`
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.NoError(t, err)
}

func TestSDLFormat_Federation_TheLinkAsRenamesTheNamespace(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", as: "fed")

type Product @fed__key(fields: "upc") {
  upc: String! @fed__shareable
}
`
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.NoError(t, err)
}

func TestSDLFormat_Federation_ARenamedImportIsUsedByItsNewName(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: [{name: "@key", as: "@primaryKey"}, "@shareable"])

type Product @primaryKey(fields: "sku") {
  upc: String! @shareable
}
`
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:3:14: @primaryKey(fields: "sku") on Product names sku, which Product doesn't define`, err.Error())
}

func TestSDLFormat_Federation_ADirectiveNotImportedIsUndefined(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

type Product @key(fields: "upc") {
  upc: String! @shareable
}
`
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:4:17: Undefined directive shareable.`, err.Error())
}

func TestSDLFormat_Federation_AVersionOtherThan2IsNotSupported(t *testing.T) {
	for _, version := range []string{"v1.0", "v3.0"} {
		sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/" + version + "\", import: [\"@key\"])\n\ntype Product @key(fields: \"upc\") {\n  upc: String!\n}\n"

		_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

		require.Error(t, err, version)
		assert.Equal(t, "schema.graphqls:1:15: federation "+version+" is not supported; supported: v2.x", err.Error())
	}
}

func TestSDLFormat_Federation_AnImportTheDefinitionSetLacks_IsAcceptedWithAnyArgumentsAnywhereAndIgnored(t *testing.T) {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", {name: "@foo", as: "@bar"}, "Baz"])
  @bar

type Query {
  product(upc: String! @bar(reason: "lookup")): Product @bar(weight: 2) @bar(weight: 3, note: "again")
}

type Product @key(fields: "upc") @bar {
  upc: String!
  status: Status
}

enum Status {
  AVAILABLE @bar
}
`
	merged := mergedFederationExtension(t, model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.Equal(t, []graphqlmodel.Directive{{Coordinate: "Product", Directive: "key", Fields: "upc", Resolvable: true}}, merged.Directives)
}

func TestSDLFormat_Federation_AnImportOfALaterVersion_IsRead(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.0\", import: [\"@interfaceObject\"])\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	assert.NoError(t, err)
}

func TestSDLFormat_Federation_AKeyNamingAFieldTheTypeLacksIsAnError(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Query {\n  product: Product\n}\n\ntype Product @key(fields: \"sku\") {\n  upc: String!\n}\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:7:14: @key(fields: "sku") on Product names sku, which Product doesn't define`, err.Error())
}

func TestSDLFormat_Federation_AKeyNamingANestedFieldItsTypeLacksIsAnError(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Product @key(fields: \"variant { sku }\") {\n  variant: Variant!\n}\n\ntype Variant {\n  id: ID!\n}\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:3:14: @key(fields: "variant { sku }") on Product names sku, which Variant doesn't define`, err.Error())
}

func TestSDLFormat_Federation_AKeyThatIsNotAFieldSetIsAnError(t *testing.T) {
	for _, fields := range []string{"id {", "", "... on Product { id }"} {
		sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Product @key(fields: \"" + fields + "\") {\n  id: ID!\n}\n"

		_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

		require.Error(t, err, fields)
		assert.Equal(t, `schema.graphqls:3:14: @key(fields: "`+fields+`") on Product is not a valid field set`, err.Error(), fields)
	}
}

func TestSDLFormat_Federation_TheFirstBadKeyInFileOrderIsTheError(t *testing.T) {
	_, err := readFederatedSDL(
		model.ContractFile{Source: "a.graphqls", Content: "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Zebra @key(fields: \"stripes\") {\n  id: ID!\n}\n"},
		model.ContractFile{Source: "b.graphqls", Content: "type Aardvark @key(fields: \"snout\") {\n  id: ID!\n}\n"},
	)

	require.Error(t, err)
	assert.Equal(t, `a.graphqls:3:12: @key(fields: "stripes") on Zebra names stripes, which Zebra doesn't define`, err.Error())
}

func TestSDLFormat_Federation_TheLinkOfOneFileServesTheOthers(t *testing.T) {
	declarations, err := readFederatedSDL(
		model.ContractFile{Source: "schema.graphqls", Content: "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n"},
		model.ContractFile{Source: "products.graphqls", Content: "type Query {\n  product: Product\n}\n\ntype Product @key(fields: \"upc\") {\n  upc: String!\n}\n"},
	)

	require.NoError(t, err)
	assert.Len(t, declarations.Resources, 2)
}

func TestSDLFormat_Federation_WithoutALinkAFederationDirectiveIsUndefined(t *testing.T) {
	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: "type Product @key(fields: \"upc\") {\n  upc: String!\n}\n"})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:1:15: Undefined directive key.`, err.Error())
}

func TestSDLFormat_Federation_AKeyWhoseOwnDefinitionLacksFieldsIsNotAFieldSet(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ndirective @key repeatable on OBJECT | INTERFACE\n\ntype Query {\n  product: Product\n}\n\ntype Product @key {\n  id: ID!\n}\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, `schema.graphqls:9:14: @key(fields: "") on Product is not a valid field set`, err.Error())
}

const federationJVMProductsSDL = `schema @link(url: "https://specs.apollo.dev/link/v1.0") @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "FieldSet"]) {
  query: Query
}

directive @federation__extends on OBJECT | INTERFACE

directive @federation__external on OBJECT | FIELD_DEFINITION

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

directive @link(as: String, for: link__Purpose, import: [link__Import], url: String!) repeatable on SCHEMA

directive @shareable repeatable on OBJECT | FIELD_DEFINITION

union _Entity = Product

type Product @key(fields: "upc") {
  upc: String!
  name: String @shareable
}

type Query {
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
  product(upc: String!): Product
}

type _Service {
  sdl: String!
}

enum link__Purpose {
  EXECUTION
  SECURITY
}

scalar FieldSet

scalar _Any

scalar federation__Policy

scalar link__Import
`

func TestSDLFormat_Federation_TheFederationDefinitionsALibraryPrints_AreNoSchemasNorResources(t *testing.T) {
	declarations, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: federationJVMProductsSDL})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"Product", "Query", "Query:product:arguments", "Query:product:returns"}, federatedSchemaNamesOf(declarations))

	var fields []string
	for _, resource := range declarations.Resources {
		fields = append(fields, resource.Resource.Identity.(graphqlmodel.Identity).Field)
	}

	assert.ElementsMatch(t, []string{"product", "product"}, fields)
}

func TestSDLFormat_WithoutAFederationLink_TypesNamedLikeTheFederationDefinitions_AreTheSDLsOwn(t *testing.T) {
	sdl := "type Query {\n  _service: _Service!\n}\n\ntype _Service {\n  sdl: String!\n}\n"

	assert.ElementsMatch(t, []string{"Query", "_Service", "Query:_service:arguments", "Query:_service:returns"}, federatedSchemaNames(t, sdl))
}

const fromContextSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.8", import: ["@key", "@context", "@fromContext"])

type Query {
  pet(id: ID!, locale: String! @fromContext(field: "$userContext { locale }")): Pet
  owner(id: ID! @fromContext(field: "$userContext { id }")): Owner
}

type Pet @key(fields: "id") @context(name: "petContext") {
  id: ID!
  owner: Owner
}

type Owner @key(fields: "id") {
  id: ID!
  greeting(name: String! @fromContext(field: "$petContext { id }"), style: String): String
  nickname(name: String! @fromContext(field: "$petContext { id }")): String
}
`

func federatedSchema(t *testing.T, declarations protocol.Declarations, name string) contract.Schema {
	t.Helper()

	for _, declaration := range declarations.Schemas {
		if declaration.Name == name {
			return declaration.Schema
		}
	}

	require.Failf(t, "schema not declared", "%s", name)

	return contract.Schema{}
}

func TestSDLFormat_Federation_AnArgumentMarkedFromContext_IsNoArgumentOfTheClient(t *testing.T) {
	declarations, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: fromContextSDL})
	require.NoError(t, err)

	assert.Equal(t, []string{"id"}, slices.Sorted(maps.Keys(federatedSchema(t, declarations, "Query:pet:arguments").Properties)))
	assert.Equal(t, contract.Schema{Type: "object"}, federatedSchema(t, declarations, "Query:owner:arguments"))
	assert.Equal(t, []string{"style"}, slices.Sorted(maps.Keys(federatedSchema(t, declarations, "Owner:greeting:arguments").Properties)))
	assert.Empty(t, federatedSchema(t, declarations, "Owner").Properties["nickname"].Arguments)
	assert.NotContains(t, federatedSchemaNamesOf(declarations), "Owner:nickname:arguments")
}
