package integration_test

import (
	"context"
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

func (this *IntegrationSuite) TestGraphQLFederationPublish_AFederation2VersionWeDontKnowIsPublished() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.16\", import: [\"@key\"])\n\ntype Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AVersionOtherThan2IsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v3.0\", import: [\"@key\"])\n\ntype Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:1:15: federation v3.0 is not supported; supported: v2.x"}`, body)
	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AnImportWeDontKnowIsPublishedAndIgnored() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\", \"@foo\"])\n\ntype Query {\n  pet: Pet @foo(weight: 2)\n}\n\ntype Pet @key(fields: \"id\") @foo {\n  id: ID!\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("graphql_federation_directives"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_ADirectiveNeitherImportedNorDefinedIsRefused() {
	sdl := "extend schema @link(url: \"https://specs.apollo.dev/federation/v2.3\", import: [\"@key\"])\n\ntype Query {\n  pet: Pet\n}\n\ntype Pet @key(fields: \"id\") {\n  id: ID! @shareable\n}\n"

	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"schema.graphqls:8:12: Undefined directive shareable."}`, body)
	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_ASubgraphWithConnectorsIsPublished() {
	sdl := `extend schema
  @link(url: "https://specs.apollo.dev/federation/v2.10", import: ["@key"])
  @link(url: "https://specs.apollo.dev/connect/v0.1", import: ["@connect", "@source"])
  @source(name: "petstore", http: {baseURL: "https://petstore.example.com/v1"})

type Query {
  pet(id: ID!): Pet @connect(source: "petstore", http: {GET: "/pets/{$args.id}"}, selection: "id name")
}

type Pet @key(fields: "id") {
  id: ID!
  name: String
}
`
	status, body := this.publishFederatedSubgraph(sdl)

	this.Equal(http.StatusOK, status, body)
	this.Equal(2, this.countRows("graphql_resources"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AFederation1SubgraphIsPublishedAndMarked() {
	sdl := `extend type Query {
  topReviews: [Review!]!
}

type Review {
  body: String
  pet: Pet
}

extend type Pet @key(fields: "id") {
  id: ID! @external
  reviews: [Review!]!
}
`
	status, body := this.publishFederatedSubgraph(sdl)
	this.Require().Equal(http.StatusOK, status, body)

	var graph string
	var federation1 bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT graph_name, federation1 FROM graphql_graph_members`).Scan(&graph, &federation1))
	this.Equal("petstore_graph", graph)
	this.True(federation1)
	this.Equal(1, this.countRows("graphql_federation_directives"))
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_AFederation2SubgraphIsNotMarkedFederation1() {
	status, body := this.publishFederatedSubgraph(federationPublishSubgraphSDL)
	this.Require().Equal(http.StatusOK, status, body)

	var federation1 bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT federation1 FROM graphql_graph_members`).Scan(&federation1))
	this.False(federation1)
}

func (this *IntegrationSuite) TestGraphQLFederationPublish_TheFederationDefinitionsALibraryPrintsAreNotPublished() {
	sdl := `schema @link(url: "https://specs.apollo.dev/link/v1.0") @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "FieldSet"]) {
  query: Query
}

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

directive @link(as: String, for: link__Purpose, import: [link__Import], url: String!) repeatable on SCHEMA

union _Entity = Pet

type Pet @key(fields: "id") {
  id: ID!
}

type Query {
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
  pet(id: ID!): Pet
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

scalar link__Import
`
	status, body := this.publishFederatedSubgraph(sdl)
	this.Require().Equal(http.StatusOK, status, body)

	this.Equal(2, this.countRows("graphql_resources"))

	rows, err := this.Pool.Query(context.Background(), `SELECT name FROM schemas ORDER BY name`)
	this.Require().NoError(err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		this.Require().NoError(rows.Scan(&name))
		names = append(names, name)
	}
	this.Require().NoError(rows.Err())

	this.Equal([]string{"Pet", "Query", "Query:pet:arguments", "Query:pet:returns"}, names)
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
