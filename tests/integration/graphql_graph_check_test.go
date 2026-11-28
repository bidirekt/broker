package integration_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/guregu/null"
)

const graphCheckGraphYAML = `provides:
  graphql:
    graph: petstore_graph
`

const graphCheckLink = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])
`

const graphCheckAPISDL = graphCheckLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
  age: Int
}
`

const graphCheckReviewsSDL = graphCheckLink + `
type Pet @key(fields: "id") {
  id: ID!
  reviews: [String!]!
}
`

const graphCheckWebReadingReviews = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, reviews: {type: array, items: {type: string}}}}
`

const graphCheckWebReadingAgeAsText = `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {age: {type: string, nullable: true}}}
`

type graphCheckResponse struct {
	Deployable bool            `json:"deployable"`
	Results    json.RawMessage `json:"results"`
}

type graphCheckRow struct {
	CounterpartName          string
	CounterpartParticipantID null.Int
	CounterpartVersion       null.String
	VerdictContractIDOne     null.Int
	Deployable               bool
}

func (this *IntegrationSuite) mustPostForGraphCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForGraphCheck(participant, version string, fragments ...contractFragment) {
	this.mustPostForGraphCheck("/api/contracts", this.publishBody(participant, version, fragments...))
}

func (this *IntegrationSuite) deployForGraphCheck(participant, version, environment string) {
	this.mustPostForGraphCheck("/api/deployments", `{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) setUpGraphCheck(participants ...string) {
	this.mustPostForGraphCheck("/api/environments", `{"environment":"production"}`)

	for _, participant := range participants {
		this.mustPostForGraphCheck("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) deployGraphCheckSubgraphs(environment string) {
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckAPISDL})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", graphCheckReviewsSDL})
	this.deployForGraphCheck("petstore_api", "1.4.0", environment)
	this.deployForGraphCheck("petstore_reviews", "2.0.0", environment)
}

func (this *IntegrationSuite) canIDeployForGraphCheck(participant, version, environment string) graphCheckResponse {
	status, body := this.post("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
	this.Require().Equal(http.StatusOK, status, body)

	var response graphCheckResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response
}

func (this *IntegrationSuite) validateForGraphCheck(participant, environment string, fragments ...contractFragment) graphCheckResponse {
	requestBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{participant, environment, fragments})
	this.Require().NoError(err)

	status, body := this.post("/api/contracts/validate", string(requestBody))
	this.Require().Equal(http.StatusOK, status, body)

	var response graphCheckResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response
}

func (this *IntegrationSuite) assertWebGraphCheck(webContract, environment string, deployable bool, results string) {
	this.publishForGraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", webContract})

	canIDeploy := this.canIDeployForGraphCheck("petstore_web", "3.0.0", environment)
	this.Equal(deployable, canIDeploy.Deployable)
	this.JSONEq(results, string(canIDeploy.Results))

	validate := this.validateForGraphCheck("petstore_web", environment, contractFragment{"web.yaml", webContract})
	this.Equal(deployable, validate.Deployable)
	this.JSONEq(results, string(validate.Results))
}

func (this *IntegrationSuite) graphCheckRows() []graphCheckRow {
	rows, err := this.Pool.Query(context.Background(),
		`SELECT counterpart_name, counterpart_participant_id, counterpart_version, verdict_contract_id_one, deployable
		   FROM compatibility_check_results
		  ORDER BY counterpart_name`)
	this.Require().NoError(err)
	defer rows.Close()

	var checkRows []graphCheckRow
	for rows.Next() {
		var row graphCheckRow
		this.Require().NoError(rows.Scan(&row.CounterpartName, &row.CounterpartParticipantID, &row.CounterpartVersion, &row.VerdictContractIDOne, &row.Deployable))
		checkRows = append(checkRows, row)
	}
	this.Require().NoError(rows.Err())

	return checkRows
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AConsumerOfAGraph_IsComparedWithTheGraphItsDeployedSubgraphsCompose() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.deployGraphCheckSubgraphs("production")

	this.assertWebGraphCheck(graphCheckWebReadingAgeAsText, "production", false, `{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"},
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.age", "consumerPropertyType": "String", "providerPropertyType": "Int", "consumerField": "Pet.age", "providerField": "Pet.age"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphWhoseSchemaNamesItsQueryRoot_GivesTheGraphItsQueries() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckLink + `
schema {
  query: RootQuery
}

type RootQuery {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  name: String!
  age: Int
}
`})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", graphCheckReviewsSDL})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")

	this.assertWebGraphCheck(graphCheckWebReadingAgeAsText, "production", false, `{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"},
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.age", "consumerPropertyType": "String", "providerPropertyType": "Int", "consumerField": "Pet.age", "providerField": "Pet.age"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeploy_AConsumerOfAGraph_RecordsTheGraphWithItsMembersAndNoVerdict() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.deployGraphCheckSubgraphs("production")
	this.publishForGraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", graphCheckWebReadingAgeAsText})

	this.canIDeployForGraphCheck("petstore_web", "3.0.0", "production")

	this.Equal([]graphCheckRow{{
		CounterpartName:    "petstore_graph",
		CounterpartVersion: null.StringFrom("petstore_api 1.4.0 + petstore_reviews 2.0.0"),
		Deployable:         false,
	}}, this.graphCheckRows())
	this.Equal(0, this.countRows("compatibility_verdicts"))
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AQueryTheGraphDoesNotResolve_IsNotFoundInTheGraph() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.deployGraphCheckSubgraphs("production")

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        owner: {arguments: OwnerArgs, returns: Owner}
schemas:
  OwnerArgs: {type: object, properties: {id: {type: string}}}
  Owner: {type: object, properties: {id: {type: string}}}
`, "production", false, `{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"},
		"operations": {"query": {"owner": {"returns": [
			{"reason": "provider_resource_not_found", "role": "consumer"}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphWithoutDeployedSubgraphs_IsNotDeployed() {
	this.setUpGraphCheck("petstore_api", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckAPISDL})

	this.assertWebGraphCheck(graphCheckWebReadingReviews, "production", false, `{"petstore_graph": {
		"deployable": false,
		"participantVersion": null,
		"operations": {"query": {"pet": {"returns": [
			{"reason": "provider_resource_not_deployed_in_environment", "role": "consumer", "details": {"graph": "petstore_graph"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AFieldOnlyAnEntitiesSubgraphResolves_BreaksWhenThatSubgraphIsNotDeployed() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.mustPostForGraphCheck("/api/environments", `{"environment":"staging"}`)
	this.deployGraphCheckSubgraphs("production")
	this.deployForGraphCheck("petstore_api", "1.4.0", "staging")

	this.assertWebGraphCheck(graphCheckWebReadingReviews, "production", true, `{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}
	}}`)

	this.assertWebGraphCheck(graphCheckWebReadingReviews, "staging", false, `{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0"},
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_graph", "property": "$.reviews", "propertyType": "[String!]!", "consumerField": "Pet.reviews"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphConsumingItsOwnGraph_IsComparedWithTheGraphItsCandidateComposes() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews")
	this.deployGraphCheckSubgraphs("production")

	candidate := []contractFragment{
		{"graph.yaml", graphCheckGraphYAML},
		{"web.yaml", `consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {nickname: {type: string}, reviews: {type: array, items: {type: string}}}}
`},
		{"api.graphqls", graphCheckLink + `
type Query {
  pet(id: ID!): Pet
}

type Pet @key(fields: "id") {
  id: ID!
  nickname: String!
}
`},
	}
	this.publishForGraphCheck("petstore_api", "1.5.0", candidate...)

	canIDeploy := this.canIDeployForGraphCheck("petstore_api", "1.5.0", "production")
	this.True(canIDeploy.Deployable)
	this.JSONEq(`{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "1.5.0", "petstore_reviews": "2.0.0"}
	}}`, string(canIDeploy.Results))

	validate := this.validateForGraphCheck("petstore_api", "production", candidate...)
	this.True(validate.Deployable)
	this.JSONEq(`{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "candidate", "petstore_reviews": "2.0.0"}
	}}`, string(validate.Results))
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphThatDoesNotCompose_BreaksEveryFieldTheConsumerReadsWithEveryCause() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_orders", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckLink + `
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
`})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", graphCheckReviewsSDL})
	this.publishForGraphCheck("petstore_orders", "1.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"orders.graphqls", graphCheckLink + `
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
`})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")
	this.deployForGraphCheck("petstore_orders", "1.0.0", "production")

	notComposable := `{"reason": "provider_graph_not_composable", "role": "consumer", "details": {"graph": "petstore_graph", "cause": "petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable", "causes": "petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable\npetstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.currency without @shareable"}}`

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        order: {arguments: OrderArgs, returns: MaybeOrder}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
  OrderArgs: {type: object, properties: {id: {type: string}}}
  MaybeOrder: {ref: Order, nullable: true}
  Order: {type: object, properties: {id: {type: string}}}
`, "production", false, `{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0", "petstore_orders": "1.0.0", "petstore_reviews": "2.0.0"},
		"operations": {"query": {
			"order": {"returns": [`+notComposable+`]},
			"pet": {"returns": [`+notComposable+`]}
		}}
	}}`)
}

func (this *IntegrationSuite) TestValidate_ACandidateThatBreaksTheComposition_IsNamedCandidateInTheCause() {
	this.setUpGraphCheck("petstore_api", "petstore_orders")
	this.publishForGraphCheck("petstore_orders", "1.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"orders.graphqls", graphCheckLink + `
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
	this.deployForGraphCheck("petstore_orders", "1.0.0", "production")

	validate := this.validateForGraphCheck("petstore_api", "production",
		contractFragment{"graph.yaml", graphCheckGraphYAML},
		contractFragment{"api.yaml", `consumes:
  petstore_graph:
    graphql:
      query:
        order: {arguments: OrderArgs, returns: MaybeOrder}
schemas:
  OrderArgs: {type: object, properties: {id: {type: string}}}
  MaybeOrder: {ref: Order, nullable: true}
  Order: {type: object, properties: {id: {type: string}}}
`},
		contractFragment{"api.graphqls", graphCheckLink + `
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
`},
	)

	this.False(validate.Deployable)
	this.JSONEq(`{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "candidate", "petstore_orders": "1.0.0"},
		"operations": {"query": {"order": {"returns": [
			{"reason": "provider_graph_not_composable", "role": "consumer", "details": {"graph": "petstore_graph", "cause": "petstore_api candidate and petstore_orders 1.0.0 both resolve Money.amount without @shareable", "causes": "petstore_api candidate and petstore_orders 1.0.0 both resolve Money.amount without @shareable"}}
		]}}}
	}}`, string(validate.Results))
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASubgraphWhoseRenamedQueryRootLeavesATypeNamedQuery_MakesTheGraphNotComposable() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckLink + `
schema {
  query: RootQuery
}

type RootQuery {
  pet(id: ID!): Pet
}

type Query {
  legacy: Int
}

type Pet @key(fields: "id") {
  id: ID!
  age: Int
}
`})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", graphCheckReviewsSDL})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")

	this.assertWebGraphCheck(graphCheckWebReadingAgeAsText, "production", false, `{"petstore_graph": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"},
		"operations": {"query": {"pet": {"returns": [
			{"reason": "provider_graph_not_composable", "role": "consumer", "details": {"graph": "petstore_graph", "cause": "petstore_api 1.4.0 has a type named Query, but its query root is RootQuery (ROOT_QUERY_USED)", "causes": "petstore_api 1.4.0 has a type named Query, but its query root is RootQuery (ROOT_QUERY_USED)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AnEntityExtendedWithItsKeyMarkedExternal_GivesTheGraphItsFields() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckAPISDL})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@external"])

type Query {
  reviews: [Review!]!
}

type Review {
  body: String!
  pet: Pet
}

extend type Pet @key(fields: "id") {
  id: ID! @external
  reviews: [Review!]!
}
`})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}, reviews: {type: array, items: {ref: Review}}}}
  Review: {type: object, properties: {body: {type: string}}}
`, "production", true, `{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}
	}}`)
}

const interfaceObjectAPISDL = `
type Query {
  animals: [Animal!]!
}

interface Animal @key(fields: "id") {
  id: ID!
  name: String!
}

type Dog implements Animal @key(fields: "id") {
  id: ID!
  name: String!
}
`

const interfaceObjectReviewsSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@interfaceObject"])

type Animal @key(fields: "id") @interfaceObject {
  id: ID!
  reviews: [String!]!
}
`

func (this *IntegrationSuite) deployInterfaceObjectGraph() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckLink + interfaceObjectAPISDL})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", interfaceObjectReviewsSDL})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AnInterfaceObject_AddsItsFieldsToTheInterface() {
	this.deployInterfaceObjectGraph()

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        animals: {returns: Animals}
schemas:
  Animals: {type: array, items: {ref: Animal}}
  Animal: {type: object, properties: {name: {type: string}, reviews: {type: array, items: {type: string}}}}
`, "production", true, `{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AnInterfaceObject_AddsItsFieldsToEveryImplementation() {
	this.deployInterfaceObjectGraph()

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        animals: {returns: Animals}
schemas:
  Animals: {type: array, items: {ref: Animal}}
  Animal: {type: union, members: {Dog: {ref: Dog}}}
  Dog: {type: object, properties: {name: {type: string}, reviews: {type: array, items: {type: string}}}}
`, "production", true, `{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphConsumerOfTheIntrospectionFields_IsDeployableAndNeverStoresThem() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.deployGraphCheckSubgraphs("production")

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        __schema: {returns: Schema}
        __type: {arguments: TypeArgs, returns: MaybeType}
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {name: {type: string}}}
  Schema: {type: object, properties: {queryType: {ref: Type}}}
  TypeArgs: {type: object, properties: {name: {type: string}}}
  MaybeType: {ref: Type, nullable: true}
  Type: {type: object, properties: {name: {type: string, nullable: true}}}
`, "production", true, `{"petstore_graph": {"deployable": true, "members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}}}`)

	var storedFields []string
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `
		SELECT COALESCE(array_agg(DISTINCT gr.field ORDER BY gr.field), '{}') FROM resources r JOIN graphql_resources gr ON gr.resource_id = r.id
		WHERE r.consumed_provider = 'petstore_graph'`).Scan(&storedFields))
	this.Equal([]string{"pet"}, storedFields)
}

const federationJVMDirectives = `
directive @federation__composeDirective(name: String!) repeatable on SCHEMA

directive @federation__extends on OBJECT | INTERFACE

directive @federation__external on OBJECT | FIELD_DEFINITION

directive @federation__interfaceObject on OBJECT

directive @federation__override(from: String!) on FIELD_DEFINITION

directive @federation__provides(fields: FieldSet!) on FIELD_DEFINITION

directive @federation__requires(fields: FieldSet!) on FIELD_DEFINITION

directive @federation__shareable repeatable on OBJECT | FIELD_DEFINITION

directive @inaccessible on SCALAR | OBJECT | FIELD_DEFINITION | ARGUMENT_DEFINITION | INTERFACE | UNION | ENUM | ENUM_VALUE | INPUT_OBJECT | INPUT_FIELD_DEFINITION

directive @key(fields: FieldSet!, resolvable: Boolean = true) repeatable on OBJECT | INTERFACE

directive @link(as: String, for: link__Purpose, import: [link__Import], url: String!) repeatable on SCHEMA

directive @tag(name: String!) repeatable on SCALAR | OBJECT | FIELD_DEFINITION | ARGUMENT_DEFINITION | INTERFACE | UNION | ENUM | ENUM_VALUE | INPUT_OBJECT | INPUT_FIELD_DEFINITION

union _Entity = Pet

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

const federationJVMAPISDL = `schema @link(import : ["@key", "FieldSet"], url : "https://specs.apollo.dev/federation/v2.3"){
  query: Query
}
` + federationJVMDirectives + `
type Pet @key(fields : "id", resolvable : true) {
  age: Int
  id: ID!
  name: String!
}

type Query {
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
  pet(id: ID!): Pet
}
`

const federationJVMReviewsSDL = `schema @link(import : ["@key", "FieldSet"], url : "https://specs.apollo.dev/federation/v2.3"){
  query: Query
}
` + federationJVMDirectives + `
type Pet @key(fields : "id", resolvable : true) {
  id: ID!
  reviews: [String!]!
}

type Query {
  _entities(representations: [_Any!]!): [_Entity]!
  _service: _Service!
}
`

func (this *IntegrationSuite) TestCanIDeploy_SubgraphsPrintedByFederationJVM_ComposeTheGraphItsConsumerReads() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", federationJVMAPISDL})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", federationJVMReviewsSDL})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")
	this.publishForGraphCheck("petstore_web", "3.0.0", contractFragment{"web.yaml", graphCheckWebReadingReviews})

	canIDeploy := this.canIDeployForGraphCheck("petstore_web", "3.0.0", "production")

	this.True(canIDeploy.Deployable)
	this.JSONEq(`{"petstore_graph": {"deployable": true, "members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}}}`, string(canIDeploy.Results))
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ASharedFieldReturningAnInterfaceAndItsImplementation_ComposesAsTheInterface() {
	this.setUpGraphCheck("petstore_api", "petstore_reviews", "petstore_web")
	this.publishForGraphCheck("petstore_api", "1.4.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"api.graphqls", graphCheckLink + `
type Query {
  pet(id: ID!): Pet
}

interface Animal {
  id: ID!
}

type Pet implements Animal @key(fields: "id") {
  id: ID!
  friend: Animal @shareable
}
`})
	this.publishForGraphCheck("petstore_reviews", "2.0.0", contractFragment{"graph.yaml", graphCheckGraphYAML}, contractFragment{"reviews.graphqls", graphCheckLink + `
interface Animal {
  id: ID!
}

type Pet implements Animal @key(fields: "id") {
  id: ID!
  friend: Pet @shareable
}
`})
	this.deployForGraphCheck("petstore_api", "1.4.0", "production")
	this.deployForGraphCheck("petstore_reviews", "2.0.0", "production")

	this.assertWebGraphCheck(`consumes:
  petstore_graph:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {friend: {ref: Friend}}}
  Friend: {type: object, nullable: true, properties: {id: {type: string}}}
`, "production", true, `{"petstore_graph": {
		"deployable": true,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"}
	}}`)
}
