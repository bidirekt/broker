package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
)

type mapReaderLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type mapReaderResult struct {
	Deployable bool                                             `json:"deployable"`
	Endpoints  map[string]map[string]map[string][]mapReaderLeaf `json:"endpoints"`
}

type mapReaderResponse struct {
	Deployable bool                       `json:"deployable"`
	Results    map[string]mapReaderResult `json:"results"`
}

const mapReaderAPIContract = `provides:
  rest:
    /api/session:
      put:
        request: OnboardingUpdate
        responses:
          200: CoreSession
schemas:
  OnboardingUpdate:
    type: object
    properties:
      onboarding:
        type: object
        values: { type: boolean }
  CoreSession:
    type: object
    properties:
      onboarding:
        type: object
        properties:
          profile: { type: boolean }
          church: { type: CHURCH_TYPE }
`

const mapReaderSPAContract = `consumes:
  session_api:
    rest:
      /api/session:
        put:
          request: OnboardingUpdate
          responses:
            200: CoreSession
schemas:
  OnboardingUpdate:
    type: object
    properties:
      onboarding:
        type: object
        properties:
          profile: { type: boolean }
          church: { type: boolean, optional: true }
  CoreSession:
    type: object
    properties:
      onboarding:
        type: object
        values: { type: boolean }
`

func mapReaderAPI(churchType string) string {
	return strings.Replace(mapReaderAPIContract, "CHURCH_TYPE", churchType, 1)
}

func (this *IntegrationSuite) mustPostForMapReader(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForMapReader(participant, version string) mapReaderResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var parsed mapReaderResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) TestCanIDeploy_MapReaderAgainstFixedKeys_ComparesEachKeyAgainstTheMapValues() {
	this.mustPostForMapReader("/api/environments", `{"environment":"production"}`)
	this.mustPostForMapReader("/api/participants", `{"participant":"session_api"}`)
	this.mustPostForMapReader("/api/participants", `{"participant":"session_spa"}`)
	this.mustPostForMapReader("/api/contracts",
		this.publishBody("session_api", "1.0.0", contractFragment{"api.yaml", mapReaderAPI("boolean")}))
	this.mustPostForMapReader("/api/deployments",
		`{"participant":"session_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForMapReader("/api/contracts",
		this.publishBody("session_spa", "1.0.0", contractFragment{"spa.yaml", mapReaderSPAContract}))
	this.mustPostForMapReader("/api/deployments",
		`{"participant":"session_spa","version":"1.0.0","environment":"production"}`)

	fromTheConsumer := this.canIDeployForMapReader("session_spa", "1.0.0")
	this.True(fromTheConsumer.Deployable)
	this.Empty(fromTheConsumer.Results["session_api"].Endpoints)

	fromTheProvider := this.canIDeployForMapReader("session_api", "1.0.0")
	this.True(fromTheProvider.Deployable)
	this.Empty(fromTheProvider.Results["session_spa"].Endpoints)

	this.mustPostForMapReader("/api/contracts",
		this.publishBody("session_api", "2.0.0", contractFragment{"api.yaml", mapReaderAPI("string")}))

	withAStringKey := this.canIDeployForMapReader("session_api", "2.0.0")
	this.False(withAStringKey.Deployable)
	this.Equal(map[string]map[string]map[string][]mapReaderLeaf{
		"/api/session": {"put": {"200": {{
			Reason: "property_type_mismatch",
			Role:   "provider",
			Details: map[string]string{
				"property":             "$.onboarding.church",
				"consumerField":        "CoreSession.onboarding{}",
				"providerField":        "CoreSession.onboarding.church",
				"consumerName":         "session_spa",
				"providerName":         "session_api",
				"consumerPropertyType": "boolean",
				"providerPropertyType": "string",
			},
		}}}},
	}, withAStringKey.Results["session_spa"].Endpoints)
}
