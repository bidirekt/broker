package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

const endpointSlashesDoubleSlashYAML = `provides:
  rest:
    //:
      get:
        responses:
          200: ~
`

const endpointSlashesRootYAML = `provides:
  rest:
    /:
      get:
        responses:
          200: ~
`

const endpointSlashesTrailingSlashYAML = `provides:
  rest:
    /pets/:
      get:
        responses:
          200: ~
`

const endpointSlashesDoubleSlashViolations = `{"message":"contract validation failed","violations":[` +
	`{"code":"endpoint.syntax","path":"provides;rest;//","source":"a.yaml","details":{"key":"//","error":"malformed path"}}` +
	`]}`

func (this *IntegrationSuite) mustPostForEndpointSlashes(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) TestPublishContract_DoubleSlashEndpoint_RejectedAsMalformed() {
	this.mustPostForEndpointSlashes("/api/participants", `{"participant":"gateway"}`)

	status, body := this.post("/api/contracts", this.publishBody("gateway", "1",
		contractFragment{"a.yaml", endpointSlashesDoubleSlashYAML},
		contractFragment{"b.yaml", endpointSlashesRootYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(endpointSlashesDoubleSlashViolations, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestValidateContract_DoubleSlashEndpoint_RejectedAsMalformed() {
	this.mustPostForEndpointSlashes("/api/participants", `{"participant":"gateway"}`)
	this.mustPostForEndpointSlashes("/api/environments", `{"environment":"production"}`)

	body, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{"gateway", "production", []contractFragment{
		{"a.yaml", endpointSlashesDoubleSlashYAML},
		{"b.yaml", endpointSlashesRootYAML},
	}})
	this.Require().NoError(err)

	status, response := this.post("/api/contracts/validate", string(body))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(endpointSlashesDoubleSlashViolations, response)
}

func (this *IntegrationSuite) TestPublishContract_RootAndTrailingSlashEndpoints_Published() {
	this.mustPostForEndpointSlashes("/api/participants", `{"participant":"gateway"}`)

	status, body := this.post("/api/contracts", this.publishBody("gateway", "1",
		contractFragment{"a.yaml", endpointSlashesRootYAML},
		contractFragment{"b.yaml", endpointSlashesTrailingSlashYAML},
	))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	var endpoints []string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT array_agg(endpoint ORDER BY endpoint COLLATE "C") FROM resources`,
	).Scan(&endpoints))
	this.Equal([]string{"/", "/pets"}, endpoints)
}
