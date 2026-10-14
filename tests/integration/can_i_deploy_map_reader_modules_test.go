package integration_test

import (
	"encoding/json"
	"net/http"
)

type mapReaderModulesLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type mapReaderModulesResult struct {
	Deployable bool                                                    `json:"deployable"`
	Endpoints  map[string]map[string]map[string][]mapReaderModulesLeaf `json:"endpoints"`
}

type mapReaderModulesResponse struct {
	Deployable bool                              `json:"deployable"`
	Results    map[string]mapReaderModulesResult `json:"results"`
}

const mapReaderModulesAPIContract = `provides:
  rest:
    /api/pets:
      put:
        request: PetUpdate
        responses:
          200: PetView
schemas:
  PetUpdate:
    type: object
    properties:
      tags:
        type: object
        values: { type: boolean }
  PetView:
    type: object
    properties:
      tags:
        type: object
        properties:
          vip: { type: boolean }
`

const mapReaderModulesAModule = `consumes:
  petstore_api:
    rest:
      /api/pets:
        put:
          request: PetASent
          responses:
            200: PetA
schemas:
  PetASent:
    type: object
    properties:
      tags: { type: object }
  PetA:
    type: object
    properties:
      tags:
        type: object
        values: { type: string }
`

const mapReaderModulesBModule = `consumes:
  petstore_api:
    rest:
      /api/pets:
        put:
          request: PetBSent
          responses:
            200: PetB
schemas:
  PetBSent:
    type: object
    properties:
      tags:
        type: object
        properties:
          vip: { type: boolean }
  PetB:
    type: object
    properties:
      tags:
        type: object
        properties:
          vip: { type: boolean }
`

func (this *IntegrationSuite) mustPostForMapReaderModules(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForMapReaderModules(participant, version string) mapReaderModulesResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var parsed mapReaderModulesResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) TestCanIDeploy_MapReaderModules_CompareEachModuleOnItsOwn() {
	this.mustPostForMapReaderModules("/api/environments", `{"environment":"production"}`)
	this.mustPostForMapReaderModules("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForMapReaderModules("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForMapReaderModules("/api/contracts",
		this.publishBody("petstore_api", "1.0.0", contractFragment{"api.yaml", mapReaderModulesAPIContract}))
	this.mustPostForMapReaderModules("/api/deployments",
		`{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForMapReaderModules("/api/contracts",
		this.publishBody("petstore_web", "1.0.0",
			contractFragment{"a.yaml", mapReaderModulesAModule},
			contractFragment{"b.yaml", mapReaderModulesBModule},
		))

	report := this.canIDeployForMapReaderModules("petstore_web", "1.0.0")
	this.False(report.Deployable)
	this.Equal(map[string]map[string]map[string][]mapReaderModulesLeaf{
		"/api/pets": {"put": {
			"request": {{
				Reason: "property_missing_in_consumer",
				Role:   "consumer",
				Details: map[string]string{
					"property":      "$.tags{}",
					"providerField": "PetUpdate.tags{}",
					"consumerName":  "petstore_web",
					"providerName":  "petstore_api",
					"propertyType":  "boolean",
				},
			}},
			"200": {{
				Reason: "property_type_mismatch",
				Role:   "consumer",
				Details: map[string]string{
					"property":             "$.tags.vip",
					"consumerField":        "PetA.tags{}",
					"providerField":        "PetView.tags.vip",
					"consumerName":         "petstore_web",
					"providerName":         "petstore_api",
					"consumerPropertyType": "string",
					"providerPropertyType": "boolean",
				},
			}},
		}},
	}, report.Results["petstore_api"].Endpoints)
}
