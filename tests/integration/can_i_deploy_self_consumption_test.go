package integration_test

import (
	"encoding/json"
	"net/http"
)

// These pin today's self-consumption behavior, not the intended one: once a self-consuming
// version is checked against its own provides, every verdict below changes.

type selfConsumptionLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type selfConsumptionEndpoints map[string]map[string]map[string][]selfConsumptionLeaf

type selfConsumptionResult struct {
	Endpoints selfConsumptionEndpoints `json:"endpoints"`
}

type selfConsumptionResponse struct {
	Deployable bool                             `json:"deployable"`
	Results    map[string]selfConsumptionResult `json:"results"`
}

const selfConsumingPetContract = `provides:
  rest:
    /pets/*:
      get:
        responses:
          200: Pet
consumes:
  petstore_bff:
    rest:
      /pets/*:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
`

const selfConsumingPetAndOwnerContract = `provides:
  rest:
    /pets/*:
      get:
        responses:
          200: Pet
    /owners/*:
      get:
        responses:
          200: Owner
consumes:
  petstore_bff:
    rest:
      /pets/*:
        get:
          responses:
            200: Pet
      /owners/*:
        get:
          responses:
            200: Owner
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
  Owner:
    type: object
    properties:
      id:
        type: string
`

const selfConsumingPetSummaryContract = `provides:
  rest:
    /pets/*:
      get:
        responses:
          200: PetSummary
consumes:
  petstore_bff:
    rest:
      /pets/*:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
  PetSummary:
    type: object
    properties:
      id:
        type: string
`

func (this *IntegrationSuite) mustPostForSelfConsumption(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForSelfConsumption(version, contract string) {
	this.mustPostForSelfConsumption("/api/contracts",
		this.publishBody("petstore_bff", version, contractFragment{"bff.yaml", contract}))
}

func (this *IntegrationSuite) canIDeployForSelfConsumption(version string) selfConsumptionResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"petstore_bff","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy petstore_bff %s: %s", version, body)

	var parsed selfConsumptionResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) selfConsumptionSetup() {
	this.mustPostForSelfConsumption("/api/environments", `{"environment":"production"}`)
	this.mustPostForSelfConsumption("/api/participants", `{"participant":"petstore_bff"}`)
}

func (this *IntegrationSuite) TestSelfConsumptionToday_FirstDeployIsBlockedAsNotDeployed() {
	this.selfConsumptionSetup()
	this.publishForSelfConsumption("1.0.0", selfConsumingPetContract)

	response := this.canIDeployForSelfConsumption("1.0.0")

	this.False(response.Deployable)
	this.Equal(map[string]selfConsumptionResult{
		"petstore_bff": {Endpoints: selfConsumptionEndpoints{
			"/pets/*": {"get": {"200": {
				{Reason: "provider_resource_not_deployed_in_environment", Role: "consumer"},
			}}},
		}},
	}, response.Results)
}

func (this *IntegrationSuite) TestSelfConsumptionToday_EndpointNewInV2IsNotFound() {
	this.selfConsumptionSetup()
	this.publishForSelfConsumption("1.0.0", selfConsumingPetContract)
	this.mustPostForSelfConsumption("/api/deployments",
		`{"participant":"petstore_bff","version":"1.0.0","environment":"production"}`)
	this.publishForSelfConsumption("2.0.0", selfConsumingPetAndOwnerContract)

	response := this.canIDeployForSelfConsumption("2.0.0")

	this.False(response.Deployable)
	this.Equal(map[string]selfConsumptionResult{
		"petstore_bff": {Endpoints: selfConsumptionEndpoints{
			"/owners/*": {"get": {"200": {
				{Reason: "provider_resource_not_found", Role: "consumer"},
			}}},
		}},
	}, response.Results)
}

func (this *IntegrationSuite) TestSelfConsumptionToday_RemovalIsBlockedByTheDeployedV1() {
	this.selfConsumptionSetup()
	this.publishForSelfConsumption("1.0.0", selfConsumingPetAndOwnerContract)
	this.mustPostForSelfConsumption("/api/deployments",
		`{"participant":"petstore_bff","version":"1.0.0","environment":"production"}`)
	this.publishForSelfConsumption("2.0.0", selfConsumingPetContract)

	response := this.canIDeployForSelfConsumption("2.0.0")

	this.False(response.Deployable)
	this.Equal(map[string]selfConsumptionResult{
		"petstore_bff": {Endpoints: selfConsumptionEndpoints{
			"/owners/*": {"get": {"200": {
				{Reason: "provider_resource_removed_but_still_consumed", Role: "provider"},
			}}},
		}},
	}, response.Results)
}

func (this *IntegrationSuite) TestSelfConsumptionToday_ReplayedSelfBreakFlipsFromProviderToConsumer() {
	this.selfConsumptionSetup()
	this.publishForSelfConsumption("1.0.0", selfConsumingPetContract)
	this.mustPostForSelfConsumption("/api/deployments",
		`{"participant":"petstore_bff","version":"1.0.0","environment":"production"}`)
	this.publishForSelfConsumption("2.0.0", selfConsumingPetSummaryContract)

	details := map[string]string{
		"property":     "$.name",
		"consumerName": "petstore_bff",
		"providerName": "petstore_bff",
		"propertyType": "string",
	}

	fresh := this.canIDeployForSelfConsumption("2.0.0")

	this.False(fresh.Deployable)
	this.Equal(map[string]selfConsumptionResult{
		"petstore_bff": {Endpoints: selfConsumptionEndpoints{
			"/pets/*": {"get": {"200": {
				{Reason: "property_missing_in_provider", Role: "provider", Details: details},
			}}},
		}},
	}, fresh.Results)

	replayed := this.canIDeployForSelfConsumption("2.0.0")

	this.False(replayed.Deployable)
	this.Equal(map[string]selfConsumptionResult{
		"petstore_bff": {Endpoints: selfConsumptionEndpoints{
			"/pets/*": {"get": {"200": {
				{Reason: "property_missing_in_provider", Role: "consumer", Details: details},
			}}},
		}},
	}, replayed.Results)
}
