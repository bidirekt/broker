package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type interactionRemovedResponse struct {
	Deployable bool                       `json:"deployable"`
	Results    map[string]json.RawMessage `json:"results"`
}

const interactionRemovedProviderV1YAML = `provides:
  rest:
    /pets:
      post:
        request: Pet
        responses:
          201: Pet
    /pets/*:
      delete:
        responses:
          200: ~
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const interactionRemovedProviderStatusChangedYAML = `provides:
  rest:
    /pets:
      post:
        request: Pet
        responses:
          201: Pet
    /pets/*:
      delete:
        responses:
          204: ~
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const interactionRemovedProviderRequestDroppedYAML = `provides:
  rest:
    /pets:
      post:
        responses:
          201: Pet
    /pets/*:
      delete:
        responses:
          200: ~
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const interactionRemovedProviderDeleteDroppedYAML = `provides:
  rest:
    /pets:
      post:
        request: Pet
        responses:
          201: Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const interactionRemovedConsumerYAML = `consumes:
  petstore_api:
    rest:
      /pets:
        post:
          request: Pet
          responses:
            201: Pet
      /pets/*:
        delete:
          responses:
            200: ~
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const interactionRemovedStatusChangedResult = `{
  "deployable": false,
  "participantVersion": "1.0.0",
  "endpoints": {
    "/pets/*": {"delete": {"200": [
      {"reason": "provider_resource_interaction_removed_but_still_consumed", "role": "provider", "details": {"providerStatuses": "204"}}
    ]}}
  }
}`

const interactionRemovedRequestDroppedResult = `{
  "deployable": false,
  "participantVersion": "1.0.0",
  "endpoints": {
    "/pets": {"post": {"request": [
      {"reason": "provider_resource_interaction_removed_but_still_consumed", "role": "provider"}
    ]}}
  }
}`

const interactionRemovedDeleteDroppedResult = `{
  "deployable": false,
  "participantVersion": "1.0.0",
  "endpoints": {
    "/pets/*": {"delete": {"200": [
      {"reason": "provider_resource_removed_but_still_consumed", "role": "provider"}
    ]}}
  }
}`

func (this *IntegrationSuite) mustPostForInteractionRemoved(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForInteractionRemoved(participant, version, content string) {
	this.mustPostForInteractionRemoved("/api/contracts",
		this.publishBody(participant, version, contractFragment{"api.yaml", content}))
}

func (this *IntegrationSuite) deployForInteractionRemoved(participant, version string) {
	this.mustPostForInteractionRemoved("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) setupForInteractionRemoved() {
	this.mustPostForInteractionRemoved("/api/environments", `{"environment":"production"}`)
	this.mustPostForInteractionRemoved("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForInteractionRemoved("/api/participants", `{"participant":"petstore_web"}`)
	this.publishForInteractionRemoved("petstore_api", "1.0.0", interactionRemovedProviderV1YAML)
	this.deployForInteractionRemoved("petstore_api", "1.0.0")
	this.publishForInteractionRemoved("petstore_web", "1.0.0", interactionRemovedConsumerYAML)
	this.deployForInteractionRemoved("petstore_web", "1.0.0")
}

func (this *IntegrationSuite) canIDeployForInteractionRemoved(version string) interactionRemovedResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"petstore_api","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s: %s", version, body)

	var response interactionRemovedResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response
}

func (this *IntegrationSuite) validateForInteractionRemoved(content string) interactionRemovedResponse {
	request, err := json.Marshal(map[string]any{
		"participant": "petstore_api",
		"environment": "production",
		"contracts":   []map[string]string{{"source": "api.yaml", "content": content}},
	})
	this.Require().NoError(err)

	status, body := this.post("/api/contracts/validate", string(request))
	this.Require().Equalf(http.StatusOK, status, "validate: %s", body)

	var response interactionRemovedResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response
}

func (this *IntegrationSuite) storedVerdictBreaksForInteractionRemoved() []string {
	rows, err := this.Pool.Query(context.Background(), `SELECT breaks::text FROM compatibility_verdicts`)
	this.Require().NoError(err)
	defer rows.Close()

	verdicts := make([]string, 0)

	for rows.Next() {
		var breaks string
		this.Require().NoError(rows.Scan(&breaks))
		verdicts = append(verdicts, breaks)
	}

	this.Require().NoError(rows.Err())

	return verdicts
}

func (this *IntegrationSuite) TestCanIDeploy_ProviderChangesTheStatusAConsumerReads_BreaksAsInteractionRemoved() {
	this.setupForInteractionRemoved()
	this.publishForInteractionRemoved("petstore_api", "2.0.0", interactionRemovedProviderStatusChangedYAML)

	response := this.canIDeployForInteractionRemoved("2.0.0")

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.JSONEq(interactionRemovedStatusChangedResult, string(response.Results["petstore_web"]))
}

func (this *IntegrationSuite) TestCanIDeploy_ProviderDropsTheRequestBodyAConsumerSends_BreaksAsInteractionRemovedWithoutDetails() {
	this.setupForInteractionRemoved()
	this.publishForInteractionRemoved("petstore_api", "2.0.0", interactionRemovedProviderRequestDroppedYAML)

	response := this.canIDeployForInteractionRemoved("2.0.0")

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.JSONEq(interactionRemovedRequestDroppedResult, string(response.Results["petstore_web"]))
}

func (this *IntegrationSuite) TestCanIDeploy_ProviderDropsTheWholeMethod_KeepsTheRemovedReason() {
	this.setupForInteractionRemoved()
	this.publishForInteractionRemoved("petstore_api", "2.0.0", interactionRemovedProviderDeleteDroppedYAML)

	response := this.canIDeployForInteractionRemoved("2.0.0")

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.JSONEq(interactionRemovedDeleteDroppedResult, string(response.Results["petstore_web"]))
}

func (this *IntegrationSuite) TestCanIDeploy_InteractionRemoved_IsNeverStoredAsAVerdict() {
	this.setupForInteractionRemoved()
	this.publishForInteractionRemoved("petstore_api", "2.0.0", interactionRemovedProviderStatusChangedYAML)

	first := this.canIDeployForInteractionRemoved("2.0.0")
	second := this.canIDeployForInteractionRemoved("2.0.0")

	this.JSONEq(interactionRemovedStatusChangedResult, string(first.Results["petstore_web"]))
	this.JSONEq(interactionRemovedStatusChangedResult, string(second.Results["petstore_web"]))

	verdicts := this.storedVerdictBreaksForInteractionRemoved()
	this.Require().Len(verdicts, 1)
	this.JSONEq(`[]`, verdicts[0])
}

func (this *IntegrationSuite) TestValidateContract_ProviderChangesTheStatusAConsumerReads_BreaksAsInteractionRemoved() {
	this.setupForInteractionRemoved()

	response := this.validateForInteractionRemoved(interactionRemovedProviderStatusChangedYAML)

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.JSONEq(interactionRemovedStatusChangedResult, string(response.Results["petstore_web"]))
	this.Zero(this.countRows("compatibility_verdicts"))
}

func (this *IntegrationSuite) TestValidateContract_ProviderDropsTheRequestBodyAConsumerSends_BreaksAsInteractionRemovedWithoutDetails() {
	this.setupForInteractionRemoved()

	response := this.validateForInteractionRemoved(interactionRemovedProviderRequestDroppedYAML)

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.JSONEq(interactionRemovedRequestDroppedResult, string(response.Results["petstore_web"]))
}

func (this *IntegrationSuite) TestValidateContract_ProviderDropsTheWholeMethod_KeepsTheRemovedReason() {
	this.setupForInteractionRemoved()

	response := this.validateForInteractionRemoved(interactionRemovedProviderDeleteDroppedYAML)

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.JSONEq(interactionRemovedDeleteDroppedResult, string(response.Results["petstore_web"]))
}
