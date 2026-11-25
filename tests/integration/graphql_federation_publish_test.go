package integration_test

import (
	"net/http"
)

const federationPublishGraphYAML = `provides:
  graphql:
    graph: petstore_graph
`

const federationPublishSubgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable", "@inaccessible", "@external", "@requires", "@tag"])

type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") @key(fields: "name owner { id }") {
  id: ID!
  name: String! @shareable
  owner: Owner!
  internalNotes: String @inaccessible
  weight: Int @external
  shippingCost: Int @requires(fields: "weight")
  nickname: String @tag(name: "public")
}

type Owner @key(fields: "id", resolvable: false) {
  id: ID!
}
`

func (this *IntegrationSuite) publishFederatedSubgraph(sdl string) (status int, body string) {
	status, _ = this.post("/api/participants", `{"participant":"petstore_api"}`)
	this.Require().Equal(http.StatusOK, status)

	return this.post("/api/contracts", this.publishBody("petstore_api", "1",
		contractFragment{"contracts/graph.yaml", federationPublishGraphYAML},
		contractFragment{"schema.graphqls", sdl},
	))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AFederatedSubgraphIsPublished() {
	status, body := this.publishFederatedSubgraph(federationPublishSubgraphSDL)

	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("graphql_resources"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_ALinkWithAsAndARenamedImportIsPublished() {
	sdl := `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", as: "fed", import: [{name: "@key", as: "@primaryKey"}])

type Query {
  pet(id: ID!): Pet
}

type Pet @primaryKey(fields: "id") {
  id: ID!
  name: String! @fed__shareable
}
`
	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AVersionNotEmbeddedIsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.99\", import: [\"@key\"])\n\ntype Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:1:15: federation v2.99 is not supported; supported: v2.0, v2.1, v2.2, v2.3, v2.4, v2.5, v2.6, v2.7, v2.8, v2.9"}`, body)
	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AnImportTheVersionDoesNotDefineIsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\", \"@foo\"])\n\ntype Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:1:15: @link imports @foo, which federation v2.3 doesn't define"}`, body)
	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AKeyNamingAFieldTheTypeLacksIsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Query {\n  product: Product\n}\n\ntype Product @key(fields: \"sku\") {\n  upc: String!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:7:14: @key(fields: \"sku\") on Product names sku, which Product doesn't define"}`, body)
	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AKeyThatIsNotAFieldSetIsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Query {\n  product: Product\n}\n\ntype Product @key(fields: \"id {\") {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:7:14: @key(fields: \"id {\") on Product is not a valid field set"}`, body)
	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AKeyWhoseOwnDefinitionLacksFieldsIsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ndirective @key repeatable on OBJECT | INTERFACE\n\ntype Query {\n  product: Product\n}\n\ntype Product @key {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:9:14: @key(fields: \"\") on Product is not a valid field set"}`, body)
	this.Equal(0, this.countRows("contracts"))
}
