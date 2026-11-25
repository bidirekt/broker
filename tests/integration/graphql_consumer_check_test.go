package integration_test

import (
	"encoding/json"
	"net/http"
)

const consumerCheckProviderSDL = `type Query {
  pet(id: ID!): Pet
  pets(first: Int! = 10): [Pet!]!
}

type Pet {
  id: ID!
}
`

const consumerCheckProviderYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      name: {type: string}
`

const consumerCheckPetConsumer = `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        pets: {returns: PetList}
schemas:
  PetArgs: {type: object, properties: {id: {type: string}}}
  MaybePet: {ref: Pet, nullable: true}
  PetList: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {id: {type: string}}}
`

type consumerCheckResponse struct {
	Deployable bool            `json:"deployable"`
	Results    json.RawMessage `json:"results"`
}

func (this *IntegrationSuite) mustPostForConsumerCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishConsumerCheckProvider() {
	this.mustPostForConsumerCheck("/api/environments", `{"environment":"production"}`)
	this.mustPostForConsumerCheck("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForConsumerCheck("/api/contracts", this.publishBody("petstore_api", "1.0.0",
		contractFragment{"api.yaml", consumerCheckProviderYAML},
		contractFragment{"schema.graphqls", consumerCheckProviderSDL},
	))
}

func (this *IntegrationSuite) deployConsumerCheckProvider() {
	this.publishConsumerCheckProvider()
	this.mustPostForConsumerCheck("/api/deployments", `{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
}

func (this *IntegrationSuite) checkFromBothEndpoints(participant, version string, contracts ...contractFragment) (canIDeploy, validate consumerCheckResponse) {
	this.mustPostForConsumerCheck("/api/contracts", this.publishBody(participant, version, contracts...))

	status, response := this.post("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, response)
	this.Require().NoError(json.Unmarshal([]byte(response), &canIDeploy))

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{Participant: participant, Environment: "production", Contracts: contracts})
	this.Require().NoError(err)

	status, response = this.post("/api/contracts/validate", string(validateBody))
	this.Require().Equal(http.StatusOK, status, response)
	this.Require().NoError(json.Unmarshal([]byte(response), &validate))

	return canIDeploy, validate
}

func (this *IntegrationSuite) assertCheckResults(participant, version string, contracts []contractFragment, deployable bool, results string) {
	canIDeploy, validate := this.checkFromBothEndpoints(participant, version, contracts...)

	this.Equal(deployable, canIDeploy.Deployable, participant)
	this.Equal(deployable, validate.Deployable, participant)
	this.JSONEq(results, string(canIDeploy.Results), participant)
	this.JSONEq(results, string(validate.Results), participant)
}

func (this *IntegrationSuite) assertConsumerCheckResults(consumer, contract string, deployable bool, results string) {
	this.mustPostForConsumerCheck("/api/participants", `{"participant":"`+consumer+`"}`)
	this.assertCheckResults(consumer, "1.0.0", []contractFragment{{"web.yaml", contract}}, deployable, results)
}

func (this *IntegrationSuite) TestCanIDeploy_AGraphQLConsumer_MatchesTheProvidedQueriesOnBothInteractions() {
	this.deployConsumerCheckProvider()

	this.mustPostForConsumerCheck("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForConsumerCheck("/api/contracts", this.publishBody("petstore_web", "1.0.0", contractFragment{"web.yaml", consumerCheckPetConsumer}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"petstore_web","version":"1.0.0","environment":"production"}`)

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{
		"message": "contract checked successfully",
		"participant": "petstore_web",
		"version": "1.0.0",
		"environment": "production",
		"deployable": true,
		"results": {"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}
	}`, body)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerOfAQueryTheProviderLacks_BreaksOnceUnderReturns() {
	this.deployConsumerCheckProvider()

	this.assertConsumerCheckResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        owner: {arguments: OwnerArgs, returns: Owner}
schemas:
  OwnerArgs: {type: object, properties: {id: {type: string}}}
  Owner: {type: object, properties: {id: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"owner": {"returns": [
			{"reason": "provider_resource_not_found", "role": "consumer"}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerSendingTheRequiredArguments_IsDeployable() {
	this.deployConsumerCheckProvider()

	this.assertConsumerCheckResults("petstore_web", consumerCheckPetConsumer, true,
		`{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumerWithoutTheRequiredArgument_BreaksUnderArguments() {
	this.deployConsumerCheckProvider()

	this.assertConsumerCheckResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pet: {returns: MaybePet}
schemas:
  MaybePet: {ref: Pet, nullable: true}
  Pet: {type: object, properties: {id: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"arguments": [
			{"reason": "property_missing_in_consumer", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "argument": "id", "propertyType": "string", "providerField": "Query.pet(id:)"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLConsumer_IsComparedWithTheSDLPetOnly() {
	this.deployConsumerCheckProvider()

	this.assertConsumerCheckResults("petstore_web", `consumes:
  petstore_api:
    graphql:
      query:
        pets: {returns: PetList}
schemas:
  PetList: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {id: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertConsumerCheckResults("petstore_admin", `consumes:
  petstore_api:
    graphql:
      query:
        pets: {returns: PetList}
schemas:
  PetList: {type: array, items: {ref: Pet}}
  Pet: {type: object, properties: {name: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pets": {"returns": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_admin", "providerName": "petstore_api", "property": "$[].name", "propertyType": "string", "consumerField": "Pet.name"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ARESTConsumer_IsComparedWithTheYAMLPetOnly() {
	this.deployConsumerCheckProvider()

	this.assertConsumerCheckResults("petstore_web", `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet: {type: object, properties: {name: {type: string}}}
`, true, `{"petstore_api": {"deployable": true, "participantVersion": "1.0.0"}}`)

	this.assertConsumerCheckResults("petstore_admin", `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet: {type: object, properties: {id: {type: string}}}
`, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"endpoints": {"/pets": {"get": {"200": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_admin", "providerName": "petstore_api", "property": "$.id", "propertyType": "string", "consumerField": "Pet.id"}}
		]}}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLProviderNotDeployed_BreaksOncePerFieldUnderReturns() {
	this.publishConsumerCheckProvider()

	this.assertConsumerCheckResults("petstore_web", consumerCheckPetConsumer, false, `{"petstore_api": {
		"deployable": false,
		"participantVersion": null,
		"operations": {"query": {
			"pet": {"returns": [{"reason": "provider_resource_not_deployed_in_environment", "role": "consumer"}]},
			"pets": {"returns": [{"reason": "provider_resource_not_deployed_in_environment", "role": "consumer"}]}
		}}
	}}`)
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_AGraphQLProviderRemovingAConsumedQuery_BreaksOnceUnderReturns() {
	this.deployConsumerCheckProvider()
	this.mustPostForConsumerCheck("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForConsumerCheck("/api/contracts", this.publishBody("petstore_web", "1.0.0", contractFragment{"web.yaml", consumerCheckPetConsumer}))
	this.mustPostForConsumerCheck("/api/deployments", `{"participant":"petstore_web","version":"1.0.0","environment":"production"}`)

	this.assertCheckResults("petstore_api", "2.0.0", []contractFragment{
		{"api.yaml", consumerCheckProviderYAML},
		{"schema.graphqls", `type Query {
  pets(first: Int! = 10): [Pet!]!
}

type Pet {
  id: ID!
}
`},
	}, false, `{"petstore_web": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"pet": {"returns": [
			{"reason": "provider_resource_removed_but_still_consumed", "role": "provider"}
		]}}}
	}}`)
}
