package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type interactionNotFoundResult struct {
	Deployable bool            `json:"deployable"`
	Endpoints  json.RawMessage `json:"endpoints"`
}

type interactionNotFoundResponse struct {
	Deployable bool                                 `json:"deployable"`
	Results    map[string]interactionNotFoundResult `json:"results"`
}

const interactionNotFoundProviderYAML = `provides:
  rest:
    /newsletter/unsubscribe:
      post:
        responses:
          204: ~
    /users/*/restore:
      post:
        responses:
          404: ~
          204: ~
    /files:
      post:
        request: ~
`

const interactionNotFoundProviderAnswering200YAML = `provides:
  rest:
    /newsletter/unsubscribe:
      post:
        responses:
          200: ~
          204: ~
`

const interactionNotFoundConsumerYAML = `consumes:
  api:
    rest:
      /newsletter/unsubscribe:
        post:
          responses:
            200: ~
        delete:
          responses:
            204: ~
      /users/*/restore:
        post:
          request: ~
          responses:
            204: ~
      /files:
        post:
          responses:
            201: ~
      /ghosts:
        get:
          responses:
            200: ~
`

const interactionNotFoundConsumerEndpoints = `{
  "/newsletter/unsubscribe": {
    "post": {"200": [{"reason": "provider_resource_interaction_not_found", "role": "consumer", "details": {"providerStatuses": "204"}}]},
    "delete": {"204": [{"reason": "provider_resource_not_found", "role": "consumer"}]}
  },
  "/users/*/restore": {
    "post": {"request": [{"reason": "provider_resource_interaction_not_found", "role": "consumer", "details": {"providerStatuses": "204, 404"}}]}
  },
  "/files": {
    "post": {"201": [{"reason": "provider_resource_interaction_not_found", "role": "consumer"}]}
  },
  "/ghosts": {
    "get": {"200": [{"reason": "provider_resource_not_found", "role": "consumer"}]}
  }
}`

const interactionNotFoundUnsubscribeConsumerYAML = `consumes:
  api:
    rest:
      /newsletter/unsubscribe:
        post:
          responses:
            200: ~
`

const interactionNotFoundPetsProviderYAML = `provides:
  rest:
    /newsletter/unsubscribe:
      post:
        responses:
          204: ~
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const interactionNotFoundPetsConsumerYAML = `consumes:
  api:
    rest:
      /newsletter/unsubscribe:
        post:
          responses:
            200: ~
      /pets:
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

func (this *IntegrationSuite) mustPostForInteractionNotFound(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) setupForInteractionNotFound(environments ...string) {
	for _, environment := range environments {
		this.mustPostForInteractionNotFound("/api/environments", `{"environment":"`+environment+`"}`)
	}

	this.mustPostForInteractionNotFound("/api/participants", `{"participant":"api"}`)
	this.mustPostForInteractionNotFound("/api/participants", `{"participant":"site"}`)
}

func (this *IntegrationSuite) publishForInteractionNotFound(participant, version, content string) {
	this.mustPostForInteractionNotFound("/api/contracts",
		this.publishBody(participant, version, contractFragment{"api.yaml", content}))
}

func (this *IntegrationSuite) deployForInteractionNotFound(participant, version, environment string) {
	this.mustPostForInteractionNotFound("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkForInteractionNotFound(path, body string) (interactionNotFoundResponse, string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)

	var parsed interactionNotFoundResponse
	this.Require().NoError(json.Unmarshal([]byte(response), &parsed))

	return parsed, response
}

func (this *IntegrationSuite) canIDeployAndValidateSiteForInteractionNotFound(content string) map[string]interactionNotFoundResponse {
	canIDeploy, _ := this.checkForInteractionNotFound("/api/can-i-deploy",
		`{"participant":"site","version":"1","environment":"production"}`)

	body, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{"site", "production", []contractFragment{{"api.yaml", content}}})
	this.Require().NoError(err)

	validate, _ := this.checkForInteractionNotFound("/api/contracts/validate", string(body))

	return map[string]interactionNotFoundResponse{"can-i-deploy": canIDeploy, "validate": validate}
}

func (this *IntegrationSuite) TestCanIDeploy_ConsumerOfAMethodTheProviderDeclaresDifferently_IsInteractionNotFound() {
	this.setupForInteractionNotFound("production")
	this.publishForInteractionNotFound("api", "1", interactionNotFoundProviderYAML)
	this.deployForInteractionNotFound("api", "1", "production")
	this.publishForInteractionNotFound("site", "1", interactionNotFoundConsumerYAML)

	for name, response := range this.canIDeployAndValidateSiteForInteractionNotFound(interactionNotFoundConsumerYAML) {
		this.False(response.Deployable, name)
		this.False(response.Results["api"].Deployable, name)
		this.JSONEq(interactionNotFoundConsumerEndpoints, string(response.Results["api"].Endpoints), name)
	}
}

func (this *IntegrationSuite) TestCanIDeploy_InteractionNotFound_ListsTheStatusesOfTheDeployedVersion() {
	this.setupForInteractionNotFound("production")
	this.publishForInteractionNotFound("api", "1", interactionNotFoundProviderYAML)
	this.deployForInteractionNotFound("api", "1", "production")
	this.publishForInteractionNotFound("api", "2", interactionNotFoundProviderAnswering200YAML)
	this.publishForInteractionNotFound("site", "1", interactionNotFoundUnsubscribeConsumerYAML)

	for name, response := range this.canIDeployAndValidateSiteForInteractionNotFound(interactionNotFoundUnsubscribeConsumerYAML) {
		this.False(response.Deployable, name)
		this.JSONEq(`{"/newsletter/unsubscribe": {"post": {"200": [
			{"reason": "provider_resource_interaction_not_found", "role": "consumer", "details": {"providerStatuses": "204"}}
		]}}}`, string(response.Results["api"].Endpoints), name)
	}
}

func (this *IntegrationSuite) TestCanIDeploy_ProviderNotDeployedInTheEnvironment_StaysProviderResourceNotFound() {
	this.setupForInteractionNotFound("production", "staging")
	this.publishForInteractionNotFound("api", "1", interactionNotFoundProviderYAML)
	this.deployForInteractionNotFound("api", "1", "staging")
	this.publishForInteractionNotFound("site", "1", interactionNotFoundUnsubscribeConsumerYAML)

	for name, response := range this.canIDeployAndValidateSiteForInteractionNotFound(interactionNotFoundUnsubscribeConsumerYAML) {
		this.False(response.Deployable, name)
		this.JSONEq(`{"/newsletter/unsubscribe": {"post": {"200": [
			{"reason": "provider_resource_not_found", "role": "consumer"}
		]}}}`, string(response.Results["api"].Endpoints), name)
	}
}

func (this *IntegrationSuite) TestCanIDeploy_InteractionNotFound_StaysLiveBesideACachedVerdict() {
	this.setupForInteractionNotFound("production")
	this.publishForInteractionNotFound("api", "1", interactionNotFoundPetsProviderYAML)
	this.deployForInteractionNotFound("api", "1", "production")
	this.publishForInteractionNotFound("site", "1", interactionNotFoundPetsConsumerYAML)

	expectedEndpoints := `{
		"/newsletter/unsubscribe": {"post": {"200": [
			{"reason": "provider_resource_interaction_not_found", "role": "consumer", "details": {"providerStatuses": "204"}}
		]}},
		"/pets": {"get": {"200": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {
				"property": "$.name", "consumerField": "Pet.name", "consumerName": "site", "providerName": "api", "propertyType": "string"
			}}
		]}}
	}`

	first, firstBody := this.checkForInteractionNotFound("/api/can-i-deploy",
		`{"participant":"site","version":"1","environment":"production"}`)
	this.False(first.Deployable)
	this.JSONEq(expectedEndpoints, string(first.Results["api"].Endpoints))

	var storedBreaks string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT breaks::text FROM compatibility_verdicts`).Scan(&storedBreaks))
	this.JSONEq(`[{"endpoint": "/pets", "method": "get", "interaction": "200", "reason": "property_missing_in_provider", "details": {
		"property": "$.name", "consumerField": "Pet.name", "consumerName": "site", "providerName": "api", "propertyType": "string"
	}}]`, storedBreaks)

	_, secondBody := this.checkForInteractionNotFound("/api/can-i-deploy",
		`{"participant":"site","version":"1","environment":"production"}`)
	this.Equal(firstBody, secondBody)

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_verdicts"))
}
