package integration_test

import (
	"context"
	"net/http"
)

const aliasProviderContract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const aliasProviderChangedContract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const aliasConsumerContract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

func (this *IntegrationSuite) TestPublish_IdenticalContentNewVersion_AliasesTheSnapshot() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/contracts", this.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/contracts", this.publishBody("api", "e4f5a6b", contractFragment{"api.yaml", aliasProviderContract}))

	// one snapshot, two version names pointing at it
	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))

	var contractIDs int
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT count(DISTINCT contract_id) FROM contract_versions`).Scan(&contractIDs))
	this.Equal(1, contractIDs)
}

func (this *IntegrationSuite) TestPublish_ChangedContentNewVersion_CreatesSnapshot() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/contracts", this.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/contracts", this.publishBody("api", "e4f5a6b", contractFragment{"api.yaml", aliasProviderChangedContract}))

	this.Equal(2, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))
}

func (this *IntegrationSuite) TestCanIDeploy_ResolvesAnAliasedVersion() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)

	// republished unchanged under a commit sha, the way CI does
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", aliasConsumerContract}))
	mustPost("/api/contracts", this.publishBody("front", "a1b2c3d", contractFragment{"api.yaml", aliasConsumerContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"a1b2c3d","environment":"production"}`)
	this.Equal(http.StatusOK, status)
	this.Contains(body, `"version":"a1b2c3d"`)
	this.Contains(body, `"deployable":true`)
}

func (this *IntegrationSuite) TestRecordDeployment_ResolvesAnAliasedVersion() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/contracts", this.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", aliasProviderContract}))

	status, _ := this.post("/api/deployments", `{"participant":"api","version":"a1b2c3d","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var deployedVersion string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT version FROM deployments LIMIT 1`).Scan(&deployedVersion))
	this.Equal("a1b2c3d", deployedVersion)
}
