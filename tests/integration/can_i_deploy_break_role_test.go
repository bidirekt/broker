package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type breakRoleLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type breakRoleResult struct {
	Endpoints map[string]map[string]map[string][]breakRoleLeaf `json:"endpoints"`
}

type breakRoleResponse struct {
	Results map[string]breakRoleResult `json:"results"`
}

const breakRoleAPIContract = `provides:
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
`

const breakRoleWebContract = `consumes:
  petstore_api:
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
      photoUrl:
        type: string
`

func (this *IntegrationSuite) mustPostForBreakRole(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForBreakRole(participant, version string) breakRoleResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var parsed breakRoleResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) TestBreakRole_ReplayedVerdictTakesTheRoleOfTheCurrentCheck() {
	this.mustPostForBreakRole("/api/environments", `{"environment":"production"}`)
	this.mustPostForBreakRole("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForBreakRole("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForBreakRole("/api/contracts",
		this.publishBody("petstore_api", "4.0.0", contractFragment{"api.yaml", breakRoleAPIContract}))
	this.mustPostForBreakRole("/api/deployments",
		`{"participant":"petstore_api","version":"4.0.0","environment":"production"}`)
	this.mustPostForBreakRole("/api/contracts",
		this.publishBody("petstore_web", "2.0.0", contractFragment{"web.yaml", breakRoleWebContract}))

	details := map[string]string{
		"property":     "$.photoUrl",
		"consumerName": "petstore_web",
		"providerName": "petstore_api",
		"propertyType": "string",
	}

	fromTheConsumer := this.canIDeployForBreakRole("petstore_web", "2.0.0")
	this.Equal([]breakRoleLeaf{{Reason: "property_missing_in_provider", Role: "consumer", Details: details}},
		fromTheConsumer.Results["petstore_api"].Endpoints["/pets/*"]["get"]["200"])

	this.mustPostForBreakRole("/api/deployments",
		`{"participant":"petstore_web","version":"2.0.0","environment":"production"}`)

	fromTheProvider := this.canIDeployForBreakRole("petstore_api", "4.0.0")
	this.Equal([]breakRoleLeaf{{Reason: "property_missing_in_provider", Role: "provider", Details: details}},
		fromTheProvider.Results["petstore_web"].Endpoints["/pets/*"]["get"]["200"])

	this.Equal(1, this.countRows("compatibility_verdicts"))

	var storedBreaks string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT breaks::text FROM compatibility_verdicts`).Scan(&storedBreaks))
	this.JSONEq(`[{
		"endpoint": "/pets/*",
		"method": "get",
		"interaction": "200",
		"reason": "property_missing_in_provider",
		"details": {
			"property": "$.photoUrl",
			"consumerName": "petstore_web",
			"providerName": "petstore_api",
			"propertyType": "string"
		}
	}]`, storedBreaks)
}
