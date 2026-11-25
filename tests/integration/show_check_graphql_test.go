package integration_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

const (
	graphqlCheckOperationsStylesheet = `<link rel="stylesheet" href="/assets/protocols/operations.css">`
	graphqlCheckEndpointsStylesheet  = `<link rel="stylesheet" href="/assets/protocols/endpoints.css">`
)

const graphqlCheckAPISDL = `type Query {
  pet(id: ID!): Pet
}

type Pet {
  id: ID!
  age: Int!
  reviews(first: Int!, filter: ReviewFilter): [String!]!
}

input ReviewFilter {
  minStars: Int
}
`

const graphqlCheckOrdersSDL = `type Query {
  orders: [String!]!
}
`

const graphqlCheckWebV1SDL = `type Query {
  stats: Int!
}
`

const graphqlCheckWebV2SDL = `type Query {
  version: String!
}
`

const graphqlCheckAdminContract = `consumes:
  petstore_web:
    graphql:
      query:
        stats: {returns: Stats}
schemas:
  Stats: {type: integer}
`

const graphqlCheckWebV2Contract = `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        owner: {returns: Owner}
  petstore_orders:
    graphql:
      query:
        orders: {returns: OrderList}
schemas:
  PetArgs: {type: object, properties: {owner: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet:
    type: object
    properties:
      age: {type: string}
      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}
  ReviewsArgs: {type: object, properties: {filter: {ref: ReviewFilter}}}
  ReviewFilter: {type: object, properties: {maxStars: {type: integer}}}
  Owner: {type: object, properties: {id: {type: string}}}
  OrderList: {type: array, items: {type: string}}
`

const graphqlCheckRESTProviderContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet: {type: object, properties: {id: {type: string}}}
`

const graphqlCheckRESTConsumerContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet: {type: object, properties: {name: {type: string}}}
`

func (this *IntegrationSuite) mustPostForGraphQLCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForGraphQLCheck(participant, version string, contracts ...contractFragment) {
	this.mustPostForGraphQLCheck("/api/contracts", this.publishBody(participant, version, contracts...))
}

func (this *IntegrationSuite) deployForGraphQLCheck(participant, version string) {
	this.mustPostForGraphQLCheck("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) checkPathForGraphQLCheck(participant, version string) string {
	this.mustPostForGraphQLCheck("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return "/checks/" + strconv.FormatInt(checkID, 10)
}

func (this *IntegrationSuite) graphqlOnlyCheckPage() string {
	this.mustPostForGraphQLCheck("/api/environments", `{"environment":"production"}`)
	for _, participant := range []string{"petstore_api", "petstore_orders", "petstore_web", "petstore_admin"} {
		this.mustPostForGraphQLCheck("/api/participants", `{"participant":"`+participant+`"}`)
	}
	this.publishForGraphQLCheck("petstore_api", "1.4.0", contractFragment{"schema.graphqls", graphqlCheckAPISDL})
	this.deployForGraphQLCheck("petstore_api", "1.4.0")
	this.publishForGraphQLCheck("petstore_orders", "1.0.0", contractFragment{"schema.graphqls", graphqlCheckOrdersSDL})
	this.publishForGraphQLCheck("petstore_web", "2.0.0", contractFragment{"schema.graphqls", graphqlCheckWebV1SDL})
	this.deployForGraphQLCheck("petstore_web", "2.0.0")
	this.publishForGraphQLCheck("petstore_admin", "1.0.0", contractFragment{"admin.yaml", graphqlCheckAdminContract})
	this.deployForGraphQLCheck("petstore_admin", "1.0.0")
	this.publishForGraphQLCheck("petstore_web", "3.0.0",
		contractFragment{"web.yaml", graphqlCheckWebV2Contract},
		contractFragment{"schema.graphqls", graphqlCheckWebV2SDL},
	)

	status, _, body := this.getPage(this.checkPathForGraphQLCheck("petstore_web", "3.0.0"), this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, body)

	return body
}

func (this *IntegrationSuite) TestShowCheck_DrawsTheBreaksOfCounterpartsThatBreakOnlyInGraphQL() {
	body := this.graphqlOnlyCheckPage()

	this.Contains(body, `<div><b>8</b><span>breaks</span></div>`)
	this.Contains(body, `<div><b>3 of 3</b><span>counterparts break</span></div>`)
	this.Contains(body, `<div><b>5</b><span>places in the contracts</span></div>`)
	this.Equal(3, strings.Count(body, `<details class="cp" open>`))
	this.NotContains(body, `Also compared with, and compatible:`)
	this.Equal(1, strings.Count(body, graphqlCheckOperationsStylesheet))
	this.NotContains(body, graphqlCheckEndpointsStylesheet)
	this.Less(strings.Index(body, `<body `), strings.Index(body, graphqlCheckOperationsStylesheet))
	this.Less(strings.Index(body, graphqlCheckOperationsStylesheet), strings.Index(body, `<nav class="crumbs"`))

	this.Contains(body, `<div class="op-title"><span class="op-kind">query</span><span>owner</span></div>`)
	this.Contains(body, `<b class="name">petstore_web</b> calls <code>query owner</code>, but <b class="name">petstore_api</b> doesn&#39;t provide it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>stop calling it</span></div><div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>publish it</span></div>`)

	this.Contains(body, `<b class="name">petstore_web</b> calls <code>query orders</code>, but <b class="name">petstore_orders</b> is not deployed in <b class="name">production</b>`)
	this.Contains(body, `<div class="fix other only"><span class="side">Fix in <b>petstore_orders</b></span><span>deploy it to production first</span></div>`)

	this.Contains(body, `<b class="name">petstore_web</b> removed <code>query stats</code>, but <b class="name">petstore_admin</b> still calls it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>keep it until petstore_admin stops calling it</span></div><div class="fix other"><span class="side">or fix in <b>petstore_admin</b></span><span>stop calling it</span></div>`)

	this.Contains(body, `<span class="op-prop">FIELD</span><span>PetArgs.owner</span><span class="badge">1 place</span>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>pet</span><span class="at">arguments</span><span class="at">(argument &#34;owner&#34;)</span></li>`)
	this.Contains(body, `<span class="op-prop">FIELD</span><span>Pet.age</span><span class="badge">1 place</span>`)
	this.Contains(body, `<b class="name">petstore_web</b> reads it as <code>string</code>, but <b class="name">petstore_api</b> provides <code>integer</code>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>pet</span><span class="at">returns</span><span>at &#34;$.age&#34;</span><span class="cf">(petstore_api: Pet.age)</span></li>`)

	this.Less(strings.Index(body, `<p class="group-label">Operations</p>`), strings.Index(body, `<p class="group-label">Fields</p>`))
}

func (this *IntegrationSuite) TestShowCheck_ANestedArgumentBreakUnderReturnsReadsAsSentArguments() {
	body := this.graphqlOnlyCheckPage()

	this.Contains(body, `<span class="op-prop">FIELD</span><span>ReviewFilter.maxStars</span><span class="badge">1 place</span>`)
	this.Contains(body, `<b class="name">petstore_web</b> sends it, but <b class="name">petstore_api</b> doesn&#39;t accept it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>stop sending it</span></div><div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>accept it</span></div>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>pet</span><span class="at">returns</span><span>at &#34;$.reviews&#34;</span><span class="at">(argument &#34;filter.maxStars&#34;)</span></li>`)

	this.Contains(body, `<b class="name">petstore_web</b> doesn&#39;t send it, but <b class="name">petstore_api</b> requires it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>send it</span></div><div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>stop requiring it, then mark it optional there</span></div>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>pet</span><span class="at">returns</span><span>at &#34;$.reviews&#34;</span><span class="at">(argument &#34;first&#34;)</span>`)

	this.NotContains(body, `reads it, but <b class="name">petstore_api</b> doesn&#39;t provide it`)
}

func (this *IntegrationSuite) TestShowCheck_AnArgumentBreakShowsTheArgumentCoordinate() {
	body := this.graphqlOnlyCheckPage()

	this.Contains(body, `<span class="op-prop">FIELD</span><span>Query.pet(id:)</span><span class="badge">1 place</span>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>pet</span><span class="at">arguments</span><span class="at">(argument &#34;id&#34;)</span><span class="cf">(petstore_api: Query.pet(id:))</span></li>`)
	this.Contains(body, `<span class="op-prop">FIELD</span><span>Pet.reviews(first:)</span><span class="badge">1 place</span>`)
	this.NotContains(body, `:arguments`)
}

func (this *IntegrationSuite) TestShowCheck_ARESTOnlyPageDoesNotLinkTheGraphQLStylesheet() {
	this.mustPostForGraphQLCheck("/api/environments", `{"environment":"production"}`)
	for _, participant := range []string{"petstore_api", "petstore_web"} {
		this.mustPostForGraphQLCheck("/api/participants", `{"participant":"`+participant+`"}`)
	}
	this.publishForGraphQLCheck("petstore_api", "1.4.0", contractFragment{"api.yaml", graphqlCheckRESTProviderContract})
	this.deployForGraphQLCheck("petstore_api", "1.4.0")
	this.publishForGraphQLCheck("petstore_web", "2.0.0", contractFragment{"web.yaml", graphqlCheckRESTConsumerContract})

	status, _, body := this.getPage(this.checkPathForGraphQLCheck("petstore_web", "2.0.0"), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, graphqlCheckEndpointsStylesheet)
	this.NotContains(body, `/assets/protocols/operations.css`)
	this.NotContains(body, `<p class="group-label">Operations</p>`)
}
