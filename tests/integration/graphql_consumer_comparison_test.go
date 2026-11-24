package integration_test

import (
	"encoding/json"
	"net/http"
)

const comparisonProviderSDL = `scalar DateTime

type Query {
  pet(id: ID!): Pet
}

type Pet {
  id: ID!
  bornAt: DateTime!
  reviews(first: Int, filter: ReviewFilter): [String!]!
}

input ReviewFilter {
  minStars: Int
}
`

type comparisonCheckResponse struct {
	Deployable bool            `json:"deployable"`
	Results    json.RawMessage `json:"results"`
}

func (this *IntegrationSuite) mustPostForComparison(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) deployComparisonProvider() {
	this.mustPostForComparison("/api/environments", `{"environment":"production"}`)
	this.mustPostForComparison("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForComparison("/api/contracts", this.publishBody("petstore_api", "1.0.0", contractFragment{"schema.graphqls", comparisonProviderSDL}))
	this.mustPostForComparison("/api/deployments", `{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
}

func (this *IntegrationSuite) assertComparisonResults(consumer, contract string, deployable bool, results string) {
	contracts := []contractFragment{{"web.yaml", contract}}

	this.mustPostForComparison("/api/participants", `{"participant":"`+consumer+`"}`)
	this.mustPostForComparison("/api/contracts", this.publishBody(consumer, "1.0.0", contracts...))

	var canIDeploy, validate comparisonCheckResponse

	status, response := this.post("/api/can-i-deploy", `{"participant":"`+consumer+`","version":"1.0.0","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, response)
	this.Require().NoError(json.Unmarshal([]byte(response), &canIDeploy))

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{Participant: consumer, Environment: "production", Contracts: contracts})
	this.Require().NoError(err)

	status, response = this.post("/api/contracts/validate", string(validateBody))
	this.Require().Equal(http.StatusOK, status, response)
	this.Require().NoError(json.Unmarshal([]byte(response), &validate))

	this.Equal(deployable, canIDeploy.Deployable, consumer)
	this.Equal(deployable, validate.Deployable, consumer)
	this.JSONEq(results, string(canIDeploy.Results), consumer)
	this.JSONEq(results, string(validate.Results), consumer)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerSendingAnArgumentTheProviderLacks_BreaksUnderArguments() {
	this.deployComparisonProvider()

	this.assertComparisonResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertComparisonResults("petstore_admin", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}, tag: {type: string, optional: true}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_admin", "providerName": "petstore_api", "property": "$.tag", "propertyType": "string", "consumerField": "PetArgs.tag"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerSendingANestedArgumentTheProviderLacks_BreaksOnTheFieldWithTheArgumentPath() {
	this.deployComparisonProvider()

	this.assertComparisonResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet:
    type: object
    properties:
      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}
  ReviewsArgs:
    type: object
    properties:
      first: {type: integer}
      filter: {ref: ReviewFilter}
  ReviewFilter: {type: object, properties: {minStars: {type: integer}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertComparisonResults("petstore_admin", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet:
    type: object
    properties:
      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}
  ReviewsArgs:
    type: object
    properties:
      filter: {ref: ReviewFilter}
  ReviewFilter: {type: object, properties: {maxStars: {type: integer}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_admin", "providerName": "petstore_api", "property": "$.reviews", "argument": "filter.maxStars", "propertyType": "integer", "consumerField": "ReviewFilter.maxStars"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerReadingAnotherScalar_BreaksUnderReturns() {
	this.deployComparisonProvider()

	this.assertComparisonResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {bornAt: {type: scalar, name: DateTime}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertComparisonResults("petstore_admin", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {bornAt: {type: scalar, name: Date}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_admin", "providerName": "petstore_api", "property": "$.bornAt", "consumerPropertyType": "Date", "providerPropertyType": "DateTime", "consumerField": "Pet.bornAt", "providerField": "Pet.bornAt"}}
		]}}}
	}}`)

	this.assertComparisonResults("petstore_mobile", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {bornAt: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_mobile", "providerName": "petstore_api", "property": "$.bornAt", "consumerPropertyType": "string", "providerPropertyType": "DateTime", "consumerField": "Pet.bornAt", "providerField": "Pet.bornAt"}}
		]}}}
	}}`)
}
