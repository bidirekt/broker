package integration_test

import (
	"encoding/json"
	"net/http"
)

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
func (s *IntegrationSuite) TestCanIDeploy_AnchorsToAnAliasedDeployedProvider() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", anchorProviderV1Contract}))
	// CI republishes the same content under the commit sha: an alias, no new snapshot
	mustPost("/api/contracts", s.publishBody("api", "a1b2c3d", contractFragment{"api.yaml", anchorProviderV1Contract}))
	mustPost("/api/deployments", `{"participant":"api","version":"a1b2c3d","environment":"production"}`)
	// a real new snapshot that was never deployed
	mustPost("/api/contracts", s.publishBody("api", "v2", contractFragment{"api.yaml", anchorProviderV2Contract}))

	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", anchorConsumerV1Contract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	s.True(got.Deployable)
	s.Require().Len(got.Results, 1)
	api := got.Results["api"]
	s.True(api.Deployable)
	s.Empty(api.Endpoints)
	s.Require().NotNil(api.ParticipantVersion)
	s.Equal("a1b2c3d", *api.ParticipantVersion)
}

func (s *IntegrationSuite) TestCanIDeploy_AnchorsToAnAliasedDeployedConsumer() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", anchorConsumerV1Contract}))
	mustPost("/api/contracts", s.publishBody("front", "a1b2c3d", contractFragment{"api.yaml", anchorConsumerV1Contract}))
	mustPost("/api/deployments", `{"participant":"front","version":"a1b2c3d","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("front", "v2", contractFragment{"api.yaml", anchorConsumerV2Contract}))

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", anchorProviderV1Contract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	s.True(got.Deployable)
	s.Require().Len(got.Results, 1)
	front := got.Results["front"]
	s.True(front.Deployable)
	s.Empty(front.Endpoints)
	s.Require().NotNil(front.ParticipantVersion)
	s.Equal("a1b2c3d", *front.ParticipantVersion)
}
