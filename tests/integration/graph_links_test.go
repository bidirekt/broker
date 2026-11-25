package integration_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

const graphLinksFederationLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key"])
`

const graphLinksAPISDL = graphLinksFederationLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
}
`

const graphLinksWebContract = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, name: {type: string}}}
`

func graphLinksGraphFile(graph string) contractFragment {
	return contractFragment{"graph.yaml", "provides:\n  graphql:\n    graph: " + graph + "\n"}
}

func (this *IntegrationSuite) mustPostForGraphLinks(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) createForGraphLinks(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForGraphLinks("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForGraphLinks("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) deployForGraphLinks(participant, version, environment string) {
	this.mustPostForGraphLinks("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkPageForGraphLinks(participant, version string) string {
	this.mustPostForGraphLinks("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	status, _, body := this.getPage("/checks/"+strconv.FormatInt(checkID, 10), this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, body)

	return body
}

func (this *IntegrationSuite) TestGraphLinks_TheSubgraphPageLinksTheGraphOfEachEnvironmentsDeployedVersion() {
	this.createForGraphLinks([]string{"development", "production", "staging"}, []string{"petstore_api"})
	this.mustPostForGraphLinks("/api/contracts", this.publishBody("petstore_api", "1.3.0",
		graphLinksGraphFile("petstore_legacy_graph"), contractFragment{"api.graphqls", graphLinksAPISDL}))
	this.mustPostForGraphLinks("/api/contracts", this.publishBody("petstore_api", "1.4.0",
		graphLinksGraphFile("petstore_graph"), contractFragment{"api.graphqls", graphLinksAPISDL}))
	this.deployForGraphLinks("petstore_api", "1.4.0", "development")
	this.deployForGraphLinks("petstore_api", "1.4.0", "production")
	this.deployForGraphLinks("petstore_api", "1.3.0", "staging")

	status, _, body := this.getPage("/participants/petstore_api", this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, body)

	this.Contains(body, `<a class="badge" href="/graphs/petstore_graph">graph petstore_graph in development</a>`+
		`<a class="badge" href="/graphs/petstore_graph">graph petstore_graph in production</a>`+
		`<a class="badge" href="/graphs/petstore_legacy_graph">graph petstore_legacy_graph in staging</a></div></div>`)
	this.Equal(3, strings.Count(body, `<a class="badge" href="/graphs/`))
}

func (this *IntegrationSuite) TestGraphLinks_TheCheckOfAGraphConsumerLinksTheGraphByName() {
	this.createForGraphLinks([]string{"production"}, []string{"petstore_api", "petstore_web"})
	this.mustPostForGraphLinks("/api/contracts", this.publishBody("petstore_api", "1.4.0",
		graphLinksGraphFile("petstore_graph"), contractFragment{"api.graphqls", graphLinksAPISDL}))
	this.mustPostForGraphLinks("/api/contracts", this.publishBody("petstore_web", "3.0.0", contractFragment{"web.yaml", graphLinksWebContract}))
	this.deployForGraphLinks("petstore_api", "1.4.0", "production")

	body := this.checkPageForGraphLinks("petstore_web", "3.0.0")

	this.Contains(body, `<a class="name" href="/graphs/petstore_graph">petstore_graph</a>`)
	this.NotContains(body, `href="/participants/petstore_graph`)
}

func (this *IntegrationSuite) TestGraphLinks_ABreakingGraphCounterpartLinksTheGraphByName() {
	this.createForGraphLinks([]string{"production"}, []string{"petstore_api", "petstore_web"})
	this.mustPostForGraphLinks("/api/contracts", this.publishBody("petstore_api", "1.4.0",
		graphLinksGraphFile("petstore_graph"), contractFragment{"api.graphqls", graphLinksAPISDL}))
	this.mustPostForGraphLinks("/api/contracts", this.publishBody("petstore_web", "3.0.0", contractFragment{"web.yaml", graphLinksWebContract}))

	body := this.checkPageForGraphLinks("petstore_web", "3.0.0")

	this.Contains(body, `</summary>
  <div class="cp-links"><a href="/graphs/petstore_graph">petstore_graph</a></div>`)
}
