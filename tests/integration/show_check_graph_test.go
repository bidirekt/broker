package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

const graphPageGraphLink = `<p class="op-see">See <a href="/graphs/petstore_graph">/graphs/petstore_graph</a></p>`

const graphPageLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])
`

const graphPageAPISDL = graphPageLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const graphPageWebContract = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, rating: {type: integer}}}
`

func graphPageGraphFile() contractFragment {
	return contractFragment{"graph.yaml", "provides:\n  graphql:\n    graph: petstore_graph\n"}
}

func graphPageReviewsSDL(petFields string) string {
	return graphPageLink + `
type Pet @key(fields: "id") {
  id: ID!
` + petFields + `}
`
}

func (this *IntegrationSuite) mustPostForGraphPage(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForGraphPage(participant, version string, contracts ...contractFragment) {
	this.mustPostForGraphPage("/api/contracts", this.publishBody(participant, version, contracts...))
}

func (this *IntegrationSuite) deployForGraphPage(participant, version string) {
	this.mustPostForGraphPage("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) setUpGraphPage(participants ...string) {
	this.mustPostForGraphPage("/api/environments", `{"environment":"production"}`)
	for _, participant := range participants {
		this.mustPostForGraphPage("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) checkPageForGraphPage(participant, version string) string {
	this.mustPostForGraphPage("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	status, _, body := this.getPage("/checks/"+strconv.FormatInt(checkID, 10), this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, body)

	return body
}

func (this *IntegrationSuite) deployGraphPageSubgraphsAndWeb() {
	this.setUpGraphPage("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphPage("petstore_api", "1.4.0", graphPageGraphFile(), contractFragment{"api.graphqls", graphPageAPISDL})
	this.publishForGraphPage("petstore_reviews", "2.0.0", graphPageGraphFile(), contractFragment{"reviews.graphqls", graphPageReviewsSDL("  rating: Int!\n")})
	this.publishForGraphPage("petstore_web", "3.0.0", contractFragment{"web.yaml", graphPageWebContract})
	this.deployForGraphPage("petstore_api", "1.4.0")
	this.deployForGraphPage("petstore_reviews", "2.0.0")
	this.deployForGraphPage("petstore_web", "3.0.0")
}

func (this *IntegrationSuite) TestShowCheck_AGraphWithoutDeployedSubgraphs_AsksToDeployASubgraphOfIt() {
	this.setUpGraphPage("petstore_api", "petstore_web")
	this.publishForGraphPage("petstore_api", "1.4.0", graphPageGraphFile(), contractFragment{"api.graphqls", graphPageAPISDL})
	this.publishForGraphPage("petstore_web", "3.0.0", contractFragment{"web.yaml", graphPageWebContract})

	body := this.checkPageForGraphPage("petstore_web", "3.0.0")

	this.Contains(body, `<div class="op-title"><span class="op-kind">query</span><span>pet</span></div>`)
	this.Contains(body, `<b class="name">petstore_web</b> calls <code>query pet</code>, but <b class="name">petstore_graph</b> is not deployed in <b class="name">production</b>`)
	this.Contains(body, `<div class="fix other only"><span class="side">Fix in <b>petstore_graph</b></span><span>deploy a subgraph of it to production first</span></div>`)
	this.NotContains(body, `deploy it to production first`)
}

func (this *IntegrationSuite) TestShowCheck_AGraphThatDoesNotCompose_ShowsTheCauseAndLinksTheGraph() {
	this.setUpGraphPage("petstore_api", "petstore_orders", "petstore_web")
	this.publishForGraphPage("petstore_api", "1.4.0", graphPageGraphFile(), contractFragment{"api.graphqls", graphPageLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  price: Money!
}

type Money {
  amount: Int!
}
`})
	this.publishForGraphPage("petstore_orders", "1.0.0", graphPageGraphFile(), contractFragment{"orders.graphqls", graphPageLink + `
type Query {
  order(id: ID!): Order
}

type Order @key(fields: "id") {
  id: ID!
  total: Money!
}

type Money @shareable {
  amount: Int!
}
`})
	this.publishForGraphPage("petstore_web", "3.0.0", contractFragment{"web.yaml", graphPageWebContract})
	this.deployForGraphPage("petstore_api", "1.4.0")
	this.deployForGraphPage("petstore_orders", "1.0.0")

	body := this.checkPageForGraphPage("petstore_web", "3.0.0")

	this.Contains(body, `<b class="name">petstore_web</b> calls <code>query pet</code>, but <b class="name">petstore_graph</b> can&#39;t be built in <b class="name">production</b>: petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable`)
	this.Contains(body, `<div class="fix other only"><span class="side">Fix in <b>petstore_graph subgraphs</b></span><span>make it compose</span></div>`)
	this.Contains(body, graphPageGraphLink)
	this.NotContains(body, `provider_graph_not_composable`)
}

func (this *IntegrationSuite) TestShowCheck_ASubgraphBreakingAConsumerOfItsGraph_SaysViaTheGraph() {
	this.deployGraphPageSubgraphsAndWeb()
	this.publishForGraphPage("petstore_reviews", "2.1.0", graphPageGraphFile(), contractFragment{"reviews.graphqls", graphPageReviewsSDL("  rating: String!\n")})

	body := this.checkPageForGraphPage("petstore_reviews", "2.1.0")

	this.Contains(body, `<span class="op-prop">FIELD</span><span>Pet.rating</span><span class="badge">1 place</span>`)
	this.Contains(body, `<b class="name">petstore_reviews</b> (via petstore_graph) provides it as <code>String!</code>, but <b class="name">petstore_web</b> reads <code>Int!</code>`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_reviews</b></span><span>provide it as Int!</span></div><div class="fix other"><span class="side">or fix in <b>petstore_web</b></span><span>read it as String! there</span></div>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>pet</span><span class="at">returns</span><span>at &#34;$.rating&#34;</span></li>`)
}

func (this *IntegrationSuite) TestShowCheck_ASubgraphThatMakesItsGraphFailToCompose_SaysSoAndLinksTheGraph() {
	this.deployGraphPageSubgraphsAndWeb()
	this.publishForGraphPage("petstore_reviews", "2.1.0", graphPageGraphFile(), contractFragment{"reviews.graphqls", graphPageReviewsSDL("  name: String!\n  rating: Int!\n")})

	body := this.checkPageForGraphPage("petstore_reviews", "2.1.0")

	this.Contains(body, `<b class="name">petstore_reviews</b> (via petstore_graph) makes <b class="name">petstore_graph</b> fail to compose in <b class="name">production</b>: petstore_api 1.4.0 and petstore_reviews 2.1.0 both resolve Pet.name without @shareable, and <b class="name">petstore_web</b> calls <code>query pet</code>`)
	this.Contains(body, `<div class="fix self only"><span class="side">Fix in <b>petstore_reviews</b></span><span>make petstore_graph compose</span></div>`)
	this.Contains(body, graphPageGraphLink)
	this.NotContains(body, `provider_graph_not_composable`)
}

func (this *IntegrationSuite) TestCanIDeployAndShowCheck_AFieldReadDirectlyAndThroughTheGraph_BreaksInBothWhenRemoved() {
	this.setUpGraphPage("petstore_api", "petstore_web")
	this.publishForGraphPage("petstore_api", "1.4.0", graphPageGraphFile(), contractFragment{"api.graphqls", graphPageAPISDL})
	this.publishForGraphPage("petstore_web", "3.0.0", contractFragment{"web.yaml", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`})
	this.deployForGraphPage("petstore_api", "1.4.0")
	this.deployForGraphPage("petstore_web", "3.0.0")
	this.publishForGraphPage("petstore_api", "1.5.0", graphPageGraphFile(), contractFragment{"api.graphqls", graphPageLink + `
type Query {
  owner(id: ID!): String
}
`})

	results := `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "provider_resource_removed_but_still_consumed", "role": "provider"},
			{"reason": "provider_resource_removed_but_still_consumed", "role": "provider", "details": {"graph": "petstore_graph"}}
		]}}}
	}}`

	status, response := this.post("/api/can-i-deploy", `{"participant":"petstore_api","version":"1.5.0","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, response)

	var canIDeploy struct {
		Results json.RawMessage `json:"results"`
	}
	this.Require().NoError(json.Unmarshal([]byte(response), &canIDeploy))
	this.JSONEq(results, string(canIDeploy.Results))

	body := this.checkPageForGraphPage("petstore_api", "1.5.0")

	var report string
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT report::text FROM compatibility_checks ORDER BY id DESC LIMIT 1`).Scan(&report))
	this.JSONEq(results, report)
	this.Contains(body, `<span><b class="name">petstore_api</b> removed <code>query pet</code>, but <b class="name">petstore_web</b> still calls it</span>`)
	this.Contains(body, `<span><b class="name">petstore_api</b> (via petstore_graph) removed <code>query pet</code>, but <b class="name">petstore_web</b> still calls it</span>`)
}
