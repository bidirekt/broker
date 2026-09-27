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

func (s *IntegrationSuite) TestPublish_IdenticalContentNewVersion_AliasesTheSnapshot() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/contracts", s.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/contracts", s.publishBody("api", "e4f5a6b", contractFragment{"api.yaml", aliasProviderContract}))

	// one snapshot, two version names pointing at it
	s.Equal(1, s.countRows("contracts"))
	s.Equal(2, s.countRows("contract_versions"))

	var contractIDs int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(DISTINCT contract_id) FROM contract_versions`).Scan(&contractIDs))
	s.Equal(1, contractIDs)
}

func (s *IntegrationSuite) TestPublish_ChangedContentNewVersion_CreatesSnapshot() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/contracts", s.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/contracts", s.publishBody("api", "e4f5a6b", contractFragment{"api.yaml", aliasProviderChangedContract}))

	s.Equal(2, s.countRows("contracts"))
	s.Equal(2, s.countRows("contract_versions"))
}

func (s *IntegrationSuite) TestCanIDeploy_ResolvesAnAliasedVersion() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)

	// republished unchanged under a commit sha, the way CI does
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", aliasConsumerContract}))
	mustPost("/api/contracts", s.publishBody("front", "a1b2c3d", contractFragment{"api.yaml", aliasConsumerContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"a1b2c3d","environment":"production"}`)
	s.Equal(http.StatusOK, status)
	s.Contains(body, `"version":"a1b2c3d"`)
	s.Contains(body, `"deployable":true`)
}

func (s *IntegrationSuite) TestRecordDeployment_ResolvesAnAliasedVersion() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", aliasProviderContract}))
	mustPost("/api/contracts", s.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", aliasProviderContract}))

	status, _ := s.post("/api/deployments", `{"participant":"api","version":"a1b2c3d","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var deployedVersion string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT version FROM deployments LIMIT 1`).Scan(&deployedVersion))
	s.Equal("a1b2c3d", deployedVersion)
}
