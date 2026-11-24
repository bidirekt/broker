package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

const subgraphCheckLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])
`

const subgraphCheckAPISDL = subgraphCheckLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
  age: Int
}
`

const subgraphCheckReviewsSDL = subgraphCheckLink + `
type Query {
  review(id: ID!): Review
  latestReviews: [String!]!
}

type Review {
  id: ID!
  body: String!
}

type Pet @key(fields: "id") {
  id: ID!
  reviews: [String!]!
}
`

const subgraphCheckWebContract = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
  petstore_reviews:
    graphql:
      query:
        review: {arguments: ReviewArgs, returns: MaybeReview}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, reviews: {type: array, items: {type: string}}}}
  ReviewArgs: {type: object, properties: {id: {type: string}}}
  MaybeReview: {ref: Review, nullable: true}
  Review: {type: object, properties: {id: {type: string}, body: {type: string}}}
`

const subgraphCheckGraph = "petstore_graph"

type subgraphCheckResponse struct {
	Deployable bool            `json:"deployable"`
	Results    json.RawMessage `json:"results"`
}

func (this *IntegrationSuite) mustPostForSubgraphCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForSubgraphCheck(participant, version string, fragments ...contractFragment) {
	this.mustPostForSubgraphCheck("/api/contracts", this.publishBody(participant, version, fragments...))
}

func (this *IntegrationSuite) deployForSubgraphCheck(participant, version string) {
	this.mustPostForSubgraphCheck("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) setUpSubgraphCheck(participants ...string) {
	this.mustPostForSubgraphCheck("/api/environments", `{"environment":"production"}`)

	for _, participant := range participants {
		this.mustPostForSubgraphCheck("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func subgraphCheckGraphFile(graph string) contractFragment {
	return contractFragment{"graph.yaml", "provides:\n  graphql:\n    graph: " + graph + "\n"}
}

func (this *IntegrationSuite) deploySubgraphCheckEnvironment() {
	this.setUpSubgraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForSubgraphCheck("petstore_api", "1.4.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"api.graphqls", subgraphCheckAPISDL})
	this.publishForSubgraphCheck("petstore_reviews", "2.0.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"reviews.graphqls", subgraphCheckReviewsSDL})
	this.publishForSubgraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", subgraphCheckWebContract})
	this.deployForSubgraphCheck("petstore_api", "1.4.0")
	this.deployForSubgraphCheck("petstore_reviews", "2.0.0")
	this.deployForSubgraphCheck("petstore_web", "3.0.0")
}

func (this *IntegrationSuite) canIDeployForSubgraphCheck(participant, version string) subgraphCheckResponse {
	status, body := this.post("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)

	var response subgraphCheckResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response
}

func (this *IntegrationSuite) validateForSubgraphCheck(participant string, fragments ...contractFragment) subgraphCheckResponse {
	requestBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{participant, "production", fragments})
	this.Require().NoError(err)

	status, body := this.post("/api/contracts/validate", string(requestBody))
	this.Require().Equal(http.StatusOK, status, body)

	var response subgraphCheckResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response
}

func (this *IntegrationSuite) assertSubgraphCheck(participant, version string, candidate []contractFragment, deployable bool, results string) {
	this.publishForSubgraphCheck(participant, version, candidate...)

	canIDeploy := this.canIDeployForSubgraphCheck(participant, version)
	this.Equal(deployable, canIDeploy.Deployable)
	this.JSONEq(results, string(canIDeploy.Results))

	validate := this.validateForSubgraphCheck(participant, candidate...)
	this.Equal(deployable, validate.Deployable)
	this.JSONEq(results, string(validate.Results))
}

func (this *IntegrationSuite) storedSubgraphCheckVerdictBreaks() []map[string]any {
	var stored []byte
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT breaks FROM compatibility_verdicts`).Scan(&stored))

	var breaks []map[string]any
	this.Require().NoError(json.Unmarshal(stored, &breaks))

	return breaks
}

const subgraphCheckReviewsWithIntegerReviews = subgraphCheckLink + `
type Query {
  review(id: ID!): Review
  latestReviews: [String!]!
}

type Review {
  id: ID!
  body: Int!
}

type Pet @key(fields: "id") {
  id: ID!
  reviews: [Int!]!
}
`

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphChangingWhatAConsumerReadsViaTheGraph_BreaksWithTheGraph() {
	this.deploySubgraphCheckEnvironment()

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckReviewsWithIntegerReviews},
	}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {
			"pet": {"returns": [
				{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.reviews[]", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Pet.reviews[]", "providerField": "Pet.reviews[]", "graph": "petstore_graph"}}
			]},
			"review": {"returns": [
				{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_reviews", "property": "$.body", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Review.body", "providerField": "Review.body"}}
			]}
		}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeploy_ASubgraphCheckRunTwice_GivesTheSameReportAndKeepsTheGraphOutOfTheVerdict() {
	this.deploySubgraphCheckEnvironment()
	this.publishForSubgraphCheck("petstore_reviews", "2.1.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"reviews.graphqls", subgraphCheckReviewsWithIntegerReviews})

	first := this.canIDeployForSubgraphCheck("petstore_reviews", "2.1.0")
	second := this.canIDeployForSubgraphCheck("petstore_reviews", "2.1.0")

	this.False(second.Deployable)
	this.JSONEq(string(first.Results), string(second.Results))
	this.Contains(string(second.Results), `"graph":"petstore_graph"`)
	this.Equal([]map[string]any{{
		"graphql": map[string]any{"operation": "query", "field": "review", "interaction": "returns"},
		"reason":  "property_type_mismatch",
		"details": map[string]any{"consumerName": "petstore_web", "providerName": "petstore_reviews", "property": "$.body", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Review.body", "providerField": "Review.body"},
	}}, this.storedSubgraphCheckVerdictBreaks())
}

const subgraphCheckReviewsOutOfTheGraph = `
type Query {
  review(id: ID!): Review
}

type Review {
  id: ID!
  body: String!
}
`

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphLeavingTheGraph_BreaksTheFieldsOnlyItResolved() {
	this.deploySubgraphCheckEnvironment()

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		{"reviews.graphqls", subgraphCheckReviewsOutOfTheGraph},
	}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_missing_in_provider", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.reviews", "propertyType": "[String!]!", "consumerField": "Pet.reviews", "graph": "petstore_graph"}}
		]}}}
	}}`)
}

const subgraphCheckAdminContract = `consumes:
  other_graph:
    graphql:
      query:
        review: {arguments: ReviewArgs, returns: MaybeReview}
schemas:
  ReviewArgs: {type: object, properties: {id: {type: string}}}
  MaybeReview: {ref: Review, nullable: true}
  Review: {type: object, properties: {id: {type: string}, rating: {type: integer}}}
`

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphSwitchingGraphs_IsCheckedInBoth() {
	this.deploySubgraphCheckEnvironment()
	this.mustPostForSubgraphCheck("/api/participants", `{"participant":"petstore_admin"}`)
	this.publishForSubgraphCheck("petstore_admin", "1.0.0", contractFragment{"admin.yaml", subgraphCheckAdminContract})
	this.deployForSubgraphCheck("petstore_admin", "1.0.0")

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		subgraphCheckGraphFile("other_graph"),
		{"reviews.graphqls", subgraphCheckReviewsSDL},
	}, false, `{
		"petstore_admin": {
			"deployable": false,
			"participantVersion": "1.0.0",
			"operations": {"query": {"review": {"returns": [
				{"reason": "property_missing_in_provider", "role": "provider", "details": {"consumerName": "petstore_admin", "providerName": "other_graph", "property": "$.rating", "propertyType": "Int!", "consumerField": "Review.rating", "graph": "other_graph"}}
			]}}}
		},
		"petstore_web": {
			"deployable": false,
			"participantVersion": "3.0.0",
			"operations": {"query": {"pet": {"returns": [
				{"reason": "property_missing_in_provider", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.reviews", "propertyType": "[String!]!", "consumerField": "Pet.reviews", "graph": "petstore_graph"}}
			]}}}
		}
	}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphEnteringTheGraph_IsComposedWithoutADeployedVersion() {
	this.setUpSubgraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForSubgraphCheck("petstore_api", "1.4.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"api.graphqls", subgraphCheckAPISDL})
	this.publishForSubgraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, reviews: {type: array, items: {type: string}}}}
`})
	this.deployForSubgraphCheck("petstore_api", "1.4.0")
	this.deployForSubgraphCheck("petstore_web", "3.0.0")

	this.assertSubgraphCheck("petstore_reviews", "2.0.0", []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckReviewsSDL},
	}, true, `{"petstore_web": {"deployable": true, "participantVersion": "3.0.0"}}`)

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckReviewsWithIntegerReviews},
	}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "3.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.reviews[]", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Pet.reviews[]", "providerField": "Pet.reviews[]", "graph": "petstore_graph"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphTakingAFieldOutOfTheGraph_IsRemovedButStillConsumed() {
	this.deploySubgraphCheckEnvironment()
	this.mustPostForSubgraphCheck("/api/participants", `{"participant":"petstore_feed"}`)
	this.publishForSubgraphCheck("petstore_feed", "1.0.0", contractFragment{"feed.yaml", `consumes:
  petstore_graph:
    graphql:
      query:
        latestReviews: {returns: Reviews}
schemas:
  Reviews: {type: array, items: {type: string}}
`})
	this.deployForSubgraphCheck("petstore_feed", "1.0.0")

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckLink + `
type Query {
  review(id: ID!): Review
}

type Review {
  id: ID!
  body: String!
}

type Pet @key(fields: "id") {
  id: ID!
  reviews: [String!]!
}
`},
	}, false, `{
		"petstore_feed": {
			"deployable": false,
			"participantVersion": "1.0.0",
			"operations": {"query": {"latestReviews": {"returns": [
				{"reason": "provider_resource_removed_but_still_consumed", "role": "provider", "details": {"graph": "petstore_graph"}}
			]}}}
		},
		"petstore_web": {"deployable": true, "participantVersion": "3.0.0"}
	}`)
}

func (this *IntegrationSuite) TestCanIDeploy_ASubgraphBreakingTheComposition_IsNotComposableForEachConsumer() {
	this.deploySubgraphCheckEnvironment()
	candidate := []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckLink + `
type Query {
  review(id: ID!): Review
  latestReviews: [String!]!
}

type Review {
  id: ID!
  body: String!
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
  reviews: [String!]!
}
`},
	}
	this.publishForSubgraphCheck("petstore_reviews", "2.1.0", candidate...)

	notComposable := func(label string) string {
		return `{"petstore_web": {
			"deployable": false,
			"participantVersion": "3.0.0",
			"operations": {"query": {"pet": {"returns": [
				{"reason": "provider_graph_not_composable", "role": "provider", "details": {"graph": "petstore_graph", "cause": "petstore_api 1.4.0 and petstore_reviews ` + label + ` both resolve Pet.name without @shareable"}}
			]}}}
		}}`
	}

	canIDeploy := this.canIDeployForSubgraphCheck("petstore_reviews", "2.1.0")
	this.False(canIDeploy.Deployable)
	this.JSONEq(notComposable("2.1.0"), string(canIDeploy.Results))

	validate := this.validateForSubgraphCheck("petstore_reviews", candidate...)
	this.False(validate.Deployable)
	this.JSONEq(notComposable("candidate"), string(validate.Results))
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ABreakTheDeployedGraphAlreadyHas_DoesNotBlockASubgraph() {
	this.setUpSubgraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForSubgraphCheck("petstore_api", "1.4.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"api.graphqls", subgraphCheckAPISDL})
	this.publishForSubgraphCheck("petstore_reviews", "2.0.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"reviews.graphqls", subgraphCheckReviewsSDL})
	this.publishForSubgraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {age: {type: string, nullable: true}}}
`})
	this.deployForSubgraphCheck("petstore_api", "1.4.0")
	this.deployForSubgraphCheck("petstore_reviews", "2.0.0")
	this.deployForSubgraphCheck("petstore_web", "3.0.0")

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckReviewsSDL},
	}, true, `{"petstore_web": {"deployable": true, "participantVersion": "3.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ACompositionErrorOfOtherSubgraphs_StaysOutOfASubgraphCheck() {
	this.setUpSubgraphCheck("petstore_api", "petstore_reviews", "petstore_orders", "petstore_web")
	this.publishForSubgraphCheck("petstore_api", "1.4.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"api.graphqls", subgraphCheckLink + `
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
	this.publishForSubgraphCheck("petstore_reviews", "2.0.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"reviews.graphqls", subgraphCheckReviewsSDL})
	this.publishForSubgraphCheck("petstore_orders", "1.0.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"orders.graphqls", subgraphCheckLink + `
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
	this.publishForSubgraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`})
	for _, deployment := range [][2]string{{"petstore_api", "1.4.0"}, {"petstore_reviews", "2.0.0"}, {"petstore_orders", "1.0.0"}, {"petstore_web", "3.0.0"}} {
		this.deployForSubgraphCheck(deployment[0], deployment[1])
	}

	this.assertSubgraphCheck("petstore_reviews", "2.1.0", []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"reviews.graphqls", subgraphCheckReviewsWithIntegerReviews},
	}, true, `{}`)
}

const subgraphCheckAPIReadingItsGraph = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {name: {type: string}, reviews: {type: array, items: {type: string}}}}
`

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphConsumingItsOwnGraph_ComparesTheOtherConsumersWithTheSameComposition() {
	this.setUpSubgraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForSubgraphCheck("petstore_api", "1.4.0",
		subgraphCheckGraphFile(subgraphCheckGraph),
		contractFragment{"api.yaml", subgraphCheckAPIReadingItsGraph},
		contractFragment{"api.graphqls", subgraphCheckAPISDL},
	)
	this.publishForSubgraphCheck("petstore_reviews", "2.0.0", subgraphCheckGraphFile(subgraphCheckGraph), contractFragment{"reviews.graphqls", subgraphCheckReviewsSDL})
	this.publishForSubgraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", subgraphCheckWebContract})
	this.deployForSubgraphCheck("petstore_api", "1.4.0")
	this.deployForSubgraphCheck("petstore_reviews", "2.0.0")
	this.deployForSubgraphCheck("petstore_web", "3.0.0")

	candidate := []contractFragment{
		subgraphCheckGraphFile(subgraphCheckGraph),
		{"api.yaml", subgraphCheckAPIReadingItsGraph},
		{"api.graphqls", subgraphCheckLink + `
type Query {
  pet(id: ID!, owner: String!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
  age: Int
}
`},
	}
	this.publishForSubgraphCheck("petstore_api", "1.5.0", candidate...)

	results := func(label string) string {
		return `{
			"petstore_graph": {
				"deployable": false,
				"members": {"petstore_api": "` + label + `", "petstore_reviews": "2.0.0"},
				"operations": {"query": {"pet": {"arguments": [
					{"reason": "property_missing_in_consumer", "role": "consumer", "details": {"consumerName": "petstore_api", "providerName": "petstore_graph", "argument": "owner", "property": "$", "propertyType": "String!", "providerField": "Query.pet(owner:)"}}
				]}}}
			},
			"petstore_web": {
				"deployable": false,
				"participantVersion": "3.0.0",
				"operations": {"query": {"pet": {"arguments": [
					{"reason": "property_missing_in_consumer", "role": "provider", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "argument": "owner", "property": "$", "propertyType": "String!", "providerField": "Query.pet(owner:)", "graph": "petstore_graph"}}
				]}}}
			}
		}`
	}

	canIDeploy := this.canIDeployForSubgraphCheck("petstore_api", "1.5.0")
	this.False(canIDeploy.Deployable)
	this.JSONEq(results("1.5.0"), string(canIDeploy.Results))

	validate := this.validateForSubgraphCheck("petstore_api", candidate...)
	this.False(validate.Deployable)
	this.JSONEq(results("candidate"), string(validate.Results))
}
