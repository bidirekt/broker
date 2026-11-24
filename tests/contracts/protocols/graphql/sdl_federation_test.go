package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
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
	return graphqlcontracts.NewSDLFormat().ToDeclarations(files)
}

func federatedSchemaNames(t *testing.T, sdl string) []string {
	t.Helper()

	declarations, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})
	require.NoError(t, err)

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

func TestSDLFormat_Federation_EveryEmbeddedVersionIsRead(t *testing.T) {
	for _, version := range []string{"v2.0", "v2.1", "v2.2", "v2.3", "v2.4", "v2.5", "v2.6", "v2.7", "v2.8", "v2.9"} {
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

func TestSDLFormat_Federation_AVersionNotEmbeddedIsNotSupported(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.99\", import: [\"@key\"])\n\ntype Product @key(fields: \"upc\") {\n  upc: String!\n}\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, "schema.graphqls:1:15: federation v2.99 is not supported; supported: v2.0, v2.1, v2.2, v2.3, v2.4, v2.5, v2.6, v2.7, v2.8, v2.9", err.Error())
}

func TestSDLFormat_Federation_AnImportTheVersionDoesNotDefineIsAnError(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\", \"@foo\"])\n\ntype Product @key(fields: \"upc\") {\n  upc: String!\n}\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, "schema.graphqls:1:15: @link imports @foo, which federation v2.3 doesn't define", err.Error())
}

func TestSDLFormat_Federation_AnImportOfALaterVersionIsAnError(t *testing.T) {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.0\", import: [\"@interfaceObject\"])\n"

	_, err := readFederatedSDL(model.ContractFile{Source: "schema.graphqls", Content: sdl})

	require.Error(t, err)
	assert.Equal(t, "schema.graphqls:1:15: @link imports @interfaceObject, which federation v2.0 doesn't define", err.Error())
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
