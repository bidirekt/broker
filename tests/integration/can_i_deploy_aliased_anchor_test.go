package integration_test

import (
	"encoding/json"
	"net/http"
)

type anchorBreak struct {
	Reason string `json:"reason"`
	Role   string `json:"role"`
}

type anchorResult struct {
	Deployable         bool                                           `json:"deployable"`
	ParticipantVersion *string                                        `json:"participantVersion"`
	Endpoints          map[string]map[string]map[string][]anchorBreak `json:"endpoints"`
}

type anchorResponse struct {
	Deployable bool                    `json:"deployable"`
	Results    map[string]anchorResult `json:"results"`
}

const anchorProviderV1Contract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const anchorProviderV2Contract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "integer" } } } }
}`

const anchorConsumerV1Contract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const anchorConsumerV2Contract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "integer" } } } }
}`

// The deployed version is an alias, not a snapshot — the anchor has to resolve it through
// contract_versions, or it falls back to MAX(id) and compares against the undeployed v2.
func (this *IntegrationSuite) TestCanIDeploy_AnchorsToAnAliasedDeployedProvider() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", anchorProviderV1Contract}))
	// CI republishes the same content under the commit sha: an alias, no new snapshot
	mustPost("/api/contracts", this.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", anchorProviderV1Contract}))
	mustPost("/api/deployments", `{"participant":"api","version":"a1b2c3d","environment":"production"}`)
	// a real new snapshot that was never deployed
	mustPost("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", anchorProviderV2Contract}))

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", anchorConsumerV1Contract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got anchorResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.True(got.Deployable)
	this.Require().Len(got.Results, 1)
	api := got.Results["api"]
	this.True(api.Deployable)
	this.Empty(api.Endpoints)
	this.Require().NotNil(api.ParticipantVersion)
	this.Equal("a1b2c3d", *api.ParticipantVersion)
}

func (this *IntegrationSuite) TestCanIDeploy_AnchorsToAnAliasedDeployedConsumer() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", anchorConsumerV1Contract}))
	mustPost("/api/contracts", this.publishBody("front", "a1b2c3d", contractFragment{"api.yaml", anchorConsumerV1Contract}))
	mustPost("/api/deployments", `{"participant":"front","version":"a1b2c3d","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("front", "v2", contractFragment{"api.yaml", anchorConsumerV2Contract}))

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", anchorProviderV1Contract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got anchorResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.True(got.Deployable)
	this.Require().Len(got.Results, 1)
	front := got.Results["front"]
	this.True(front.Deployable)
	this.Empty(front.Endpoints)
	this.Require().NotNil(front.ParticipantVersion)
	this.Equal("a1b2c3d", *front.ParticipantVersion)
}

const anchorProviderWithPetsContract = `
{
  "provides": { "rest": { "/pets": { "get": { "responses": { "200": "Pet" } } } } },
  "schemas": { "Pet": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const anchorProviderWithoutPetsContract = `
{
  "provides": { "rest": { "/owners": { "get": { "responses": { "200": "Owner" } } } } },
  "schemas": { "Owner": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const anchorConsumerOfPetsContract = `
{
  "consumes": { "api": { "rest": { "/pets": { "get": { "responses": { "200": "Pet" } } } } } },
  "schemas": { "Pet": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

func (this *IntegrationSuite) TestCanIDeploy_UndeployedProviderWhoseLatestVersionReusesAnOlderSnapshot_IsNotDeployed() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", anchorProviderWithPetsContract}))
	mustPost("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", anchorProviderWithoutPetsContract}))
	mustPost("/api/contracts", this.publishBody("api", "v3", contractFragment{"api.yaml", anchorProviderWithPetsContract}))

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", anchorConsumerOfPetsContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got anchorResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.False(got.Deployable)
	api := got.Results["api"]
	this.False(api.Deployable)
	breaks := api.Endpoints["/pets"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("provider_resource_not_deployed_in_environment", breaks[0].Reason)
	this.Equal("consumer", breaks[0].Role)
}

func (this *IntegrationSuite) TestCanIDeploy_UndeployedProviderWhoseLatestVersionDropsTheResource_IsNotFound() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", anchorProviderWithPetsContract}))
	mustPost("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", anchorProviderWithoutPetsContract}))

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", anchorConsumerOfPetsContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got anchorResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.False(got.Deployable)
	api := got.Results["api"]
	this.False(api.Deployable)
	breaks := api.Endpoints["/pets"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("provider_resource_not_found", breaks[0].Reason)
	this.Equal("consumer", breaks[0].Role)
}
