package integration_test

import (
	"net/http"
	"regexp"
)

const showGraphLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])
`

const showGraphAPISDL = showGraphLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const showGraphReviewsSDL = showGraphLink + `
type Pet @key(fields: "id") {
  id: ID!
  rating: Int!
}
`

const showGraphAPIWithMoneySDL = showGraphLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  price: Money!
}

type Money {
  amount: Int!
  currency: String!
}
`

const showGraphOrdersSDL = showGraphLink + `
type Query {
  order(id: ID!): Order
}

type Order @key(fields: "id") {
  id: ID!
  total: Money!
}

type Money @shareable {
  amount: Int!
  currency: String!
}
`

const showGraphFederationJVMSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])

schema {
  query: RootQuery
}

directive @link(url: String!, as: String, for: link__Purpose, import: [link__Import]) repeatable on SCHEMA

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

scalar link__Import

enum link__Purpose {
  SECURITY
  EXECUTION
}

scalar FieldSet

type RootQuery {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  rating: Int
}
`

const showGraphOverridingReviewsSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@override"])

type Pet @key(fields: "id") {
  id: ID!
  rating: Int @override(from: "petstore_api")
}
`

const (
	showGraphComposes       = `<span class="badge yes">`
	showGraphDoesNotCompose = `<span class="badge no">`
	showGraphNoSubgraph     = `<p class="pad muted">no subgraph deployed</p>`
)

func showGraphGraphFile() contractFragment {
	return contractFragment{"graph.yaml", "provides:\n  graphql:\n    graph: petstore_graph\n"}
}

func showGraphMemberRow(participant, version string) string {
	return `<td data-l="Subgraph"><a class="name" href="/participants/` + participant + `">` + participant + `</a></td>
        <td data-l="Version"><a class="ver" href="/participants/` + participant + `/versions/` + version + `">` + version + `</a></td>`
}

func (this *IntegrationSuite) mustPostForShowGraph(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createForShowGraph(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForShowGraph("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForShowGraph("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) publishSubgraphForShowGraph(participant, version, source, sdl string) {
	this.mustPostForShowGraph("/api/contracts", this.publishBody(participant, version, showGraphGraphFile(), contractFragment{source, sdl}))
}

func (this *IntegrationSuite) deployForShowGraph(participant, version, environment string) {
	this.mustPostForShowGraph("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) graphPageForShowGraph() string {
	status, _, body := this.getPage("/graphs/petstore_graph", this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, body)

	return body
}

func (this *IntegrationSuite) environmentSectionForShowGraph(body, environment string) string {
	section := regexp.MustCompile(`(?s)<section class="sheet">\s*<div class="sheet-head"><h2 class="h2"><a class="name" href="/environments/\d+">` +
		environment + `</a></h2>.*?</section>`).FindString(body)
	this.Require().NotEmpty(section, "no section for environment %s", environment)

	return section
}

func (this *IntegrationSuite) TestShowGraph_ShowsEachEnvironmentWithItsDeployedSubgraphsAndTheirComposition() {
	this.createForShowGraph([]string{"production", "staging"}, []string{"petstore_api", "petstore_reviews"})
	this.publishSubgraphForShowGraph("petstore_api", "1.3.0", "api.graphqls", showGraphAPISDL)
	this.publishSubgraphForShowGraph("petstore_api", "1.4.0", "api.graphqls", showGraphAPISDL+"\ntype Owner {\n  id: ID!\n}\n")
	this.publishSubgraphForShowGraph("petstore_reviews", "2.0.0", "reviews.graphqls", showGraphReviewsSDL)
	this.deployForShowGraph("petstore_api", "1.4.0", "production")
	this.deployForShowGraph("petstore_reviews", "2.0.0", "production")
	this.deployForShowGraph("petstore_api", "1.3.0", "staging")

	body := this.graphPageForShowGraph()

	this.Contains(body, `<h1 class="mono">petstore_graph</h1>`)

	production := this.environmentSectionForShowGraph(body, "production")
	this.Contains(production, showGraphComposes)
	this.Contains(production, showGraphMemberRow("petstore_api", "1.4.0"))
	this.Contains(production, showGraphMemberRow("petstore_reviews", "2.0.0"))

	staging := this.environmentSectionForShowGraph(body, "staging")
	this.Contains(staging, showGraphComposes)
	this.Contains(staging, showGraphMemberRow("petstore_api", "1.3.0"))
	this.NotContains(staging, "petstore_reviews")

	this.Less(regexp.MustCompile(`>production</a></h2>`).FindStringIndex(body)[0], regexp.MustCompile(`>staging</a></h2>`).FindStringIndex(body)[0])
}

func (this *IntegrationSuite) TestShowGraph_AnEnvironmentWithoutDeployedSubgraphs_SaysNoSubgraphDeployed() {
	this.createForShowGraph([]string{"production", "staging"}, []string{"petstore_api"})
	this.publishSubgraphForShowGraph("petstore_api", "1.4.0", "api.graphqls", showGraphAPISDL)
	this.deployForShowGraph("petstore_api", "1.4.0", "production")

	body := this.graphPageForShowGraph()

	staging := this.environmentSectionForShowGraph(body, "staging")
	this.Contains(staging, showGraphNoSubgraph)
	this.NotContains(staging, showGraphComposes)
	this.NotContains(staging, showGraphDoesNotCompose)

	this.NotContains(this.environmentSectionForShowGraph(body, "production"), showGraphNoSubgraph)
}

func (this *IntegrationSuite) TestShowGraph_AGraphThatDoesNotCompose_ListsEveryError() {
	this.createForShowGraph([]string{"production"}, []string{"petstore_api", "petstore_orders"})
	this.publishSubgraphForShowGraph("petstore_api", "1.4.0", "api.graphqls", showGraphAPIWithMoneySDL)
	this.publishSubgraphForShowGraph("petstore_orders", "1.0.0", "orders.graphqls", showGraphOrdersSDL)
	this.deployForShowGraph("petstore_api", "1.4.0", "production")
	this.deployForShowGraph("petstore_orders", "1.0.0", "production")

	production := this.environmentSectionForShowGraph(this.graphPageForShowGraph(), "production")

	this.Contains(production, showGraphDoesNotCompose)
	this.Contains(production, showGraphMemberRow("petstore_api", "1.4.0"))
	this.Contains(production, showGraphMemberRow("petstore_orders", "1.0.0"))
	this.Contains(production, `<ul class="list">
    <li>petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable</li><li>petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.currency without @shareable</li>
  </ul>`)
}

func (this *IntegrationSuite) TestShowGraph_MembersWithAnOverrideACustomRootAndDeclaredFederationDefinitions_Compose() {
	this.createForShowGraph([]string{"production"}, []string{"petstore_api", "petstore_reviews"})
	this.publishSubgraphForShowGraph("petstore_api", "1.4.0", "api.graphqls", showGraphFederationJVMSDL)
	this.publishSubgraphForShowGraph("petstore_reviews", "2.0.0", "reviews.graphqls", showGraphOverridingReviewsSDL)
	this.deployForShowGraph("petstore_api", "1.4.0", "production")
	this.deployForShowGraph("petstore_reviews", "2.0.0", "production")

	production := this.environmentSectionForShowGraph(this.graphPageForShowGraph(), "production")

	this.Contains(production, showGraphComposes)
	this.NotContains(production, `<ul class="list">`)
	this.Contains(production, showGraphMemberRow("petstore_api", "1.4.0"))
	this.Contains(production, showGraphMemberRow("petstore_reviews", "2.0.0"))
	for _, definition := range []string{"FieldSet", "link__Import", "link__Purpose", "RootQuery"} {
		this.NotContains(production, definition)
	}
}

func (this *IntegrationSuite) TestShowGraph_ANameWithoutPublishedSubgraphsIsNotFound() {
	this.createForShowGraph([]string{"production"}, []string{"petstore_api"})
	this.publishSubgraphForShowGraph("petstore_api", "1.4.0", "api.graphqls", showGraphAPISDL)

	for _, path := range []string{"/graphs/unknown_graph", "/graphs/petstore_api", "/graphs/Petstore_Graph"} {
		status, _, _ := this.getPage(path, this.signInAsAdmin())
		this.Equal(http.StatusNotFound, status, path)
	}
}
