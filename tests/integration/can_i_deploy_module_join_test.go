package integration_test

import (
	"encoding/json"
	"net/http"
)

type moduleJoinLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type moduleJoinResult struct {
	Deployable bool                                              `json:"deployable"`
	Endpoints  map[string]map[string]map[string][]moduleJoinLeaf `json:"endpoints"`
}

type moduleJoinResponse struct {
	Deployable bool                        `json:"deployable"`
	Results    map[string]moduleJoinResult `json:"results"`
}

const moduleJoinProviderContract = `provides:
  rest:
    /api/groups:
      post:
        request: GroupInput
schemas:
  GroupInput:
    type: object
    properties:
      name: { type: string }
      address:
        type: object
        optional: true
        properties:
          city: { type: string }
`

const moduleJoinAddressModule = `consumes:
  groups_api:
    rest:
      /api/groups:
        post:
          request: GroupWithAddress
schemas:
  GroupWithAddress:
    type: object
    properties:
      name: { type: string }
      address:
        type: object
        properties:
          city: { type: string }
`

const moduleJoinNameModule = `consumes:
  groups_api:
    rest:
      /api/groups:
        post:
          request: GroupName
schemas:
  GroupName:
    type: object
    properties:
      name: { type: string }
`

const moduleJoinNullableNameModule = `consumes:
  groups_api:
    rest:
      /api/groups:
        post:
          request: GroupNullableName
schemas:
  GroupNullableName:
    type: object
    properties:
      name: { type: string, nullable: true }
`

func (this *IntegrationSuite) mustPostForModuleJoin(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) checkForModuleJoin(path, body string) moduleJoinResponse {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)

	var parsed moduleJoinResponse
	this.Require().NoError(json.Unmarshal([]byte(response), &parsed))

	return parsed
}

func (this *IntegrationSuite) checkModuleJoinFromBothEndpoints(modules ...contractFragment) (canIDeploy, validate moduleJoinResponse) {
	this.mustPostForModuleJoin("/api/environments", `{"environment":"production"}`)
	this.mustPostForModuleJoin("/api/participants", `{"participant":"groups_api"}`)
	this.mustPostForModuleJoin("/api/participants", `{"participant":"groups_spa"}`)
	this.mustPostForModuleJoin("/api/contracts",
		this.publishBody("groups_api", "1.0.0", contractFragment{"api.yaml", moduleJoinProviderContract}))
	this.mustPostForModuleJoin("/api/deployments",
		`{"participant":"groups_api","version":"1.0.0","environment":"production"}`)

	this.mustPostForModuleJoin("/api/contracts", this.publishBody("groups_spa", "1.0.0", modules...))

	canIDeploy = this.checkForModuleJoin("/api/can-i-deploy",
		`{"participant":"groups_spa","version":"1.0.0","environment":"production"}`)

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{Participant: "groups_spa", Environment: "production", Contracts: modules})
	this.Require().NoError(err)

	validate = this.checkForModuleJoin("/api/contracts/validate", string(validateBody))

	return canIDeploy, validate
}

func (this *IntegrationSuite) TestCanIDeploy_ModulesSendingTheAddressOnlySometimes_AreDeployableFromStoredAndUploadedBodies() {
	canIDeploy, validate := this.checkModuleJoinFromBothEndpoints(
		contractFragment{"address.yaml", moduleJoinAddressModule},
		contractFragment{"name.yaml", moduleJoinNameModule},
	)

	this.True(canIDeploy.Deployable)
	this.Empty(canIDeploy.Results["groups_api"].Endpoints)

	this.True(validate.Deployable)
	this.Empty(validate.Results["groups_api"].Endpoints)
}

func (this *IntegrationSuite) TestCanIDeploy_ModuleSendingANullableName_BreaksOnceFromStoredAndUploadedBodies() {
	canIDeploy, validate := this.checkModuleJoinFromBothEndpoints(
		contractFragment{"address.yaml", moduleJoinAddressModule},
		contractFragment{"name.yaml", moduleJoinNullableNameModule},
	)

	expected := map[string]map[string]map[string][]moduleJoinLeaf{
		"/api/groups": {"post": {"request": {{
			Reason: "property_nullable_in_consumer_not_nullable_in_provider",
			Role:   "consumer",
			Details: map[string]string{
				"property":     "$.name",
				"consumerName": "groups_spa",
				"providerName": "groups_api",
				"propertyType": "string",
			},
		}}}},
	}

	this.False(canIDeploy.Deployable)
	this.Equal(expected, canIDeploy.Results["groups_api"].Endpoints)

	this.False(validate.Deployable)
	this.Equal(expected, validate.Results["groups_api"].Endpoints)
}
