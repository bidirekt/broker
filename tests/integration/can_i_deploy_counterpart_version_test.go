package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type counterpartVersionResult struct {
	ParticipantVersion *string                                                   `json:"participantVersion"`
	Endpoints          map[string]map[string]map[string][]counterpartVersionLeaf `json:"endpoints"`
}

type counterpartVersionLeaf struct {
	Reason  string            `json:"reason"`
	Details map[string]string `json:"details"`
}

type counterpartVersionResponse struct {
	Results map[string]counterpartVersionResult `json:"results"`
}

type counterpartVersionRow struct {
	ParticipantID *int64
	Version       *string
}

const counterpartVersionProviderYAML = `provides:
  rest:
    /orders:
      get:
        responses:
          204: ~
`

const counterpartVersionMissingEndpointYAML = `consumes:
  orders:
    rest:
      /pets:
        get:
          responses:
            200: ~
`

const counterpartVersionMissingStatusYAML = `consumes:
  orders:
    rest:
      /orders:
        get:
          responses:
            200: ~
`

func (this *IntegrationSuite) mustPostForCounterpartVersion(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) setupForCounterpartVersion() {
	this.mustPostForCounterpartVersion("/api/environments", `{"environment":"production"}`)
	this.mustPostForCounterpartVersion("/api/environments", `{"environment":"staging"}`)
	this.mustPostForCounterpartVersion("/api/participants", `{"participant":"orders"}`)
	this.mustPostForCounterpartVersion("/api/participants", `{"participant":"front"}`)
}

func (this *IntegrationSuite) publishForCounterpartVersion(participant, version, content string) {
	this.mustPostForCounterpartVersion("/api/contracts",
		this.publishBody(participant, version, contractFragment{"api.yaml", content}))
}

func (this *IntegrationSuite) deployForCounterpartVersion(participant, version, environment string) {
	this.mustPostForCounterpartVersion("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) checkFrontForCounterpartVersion(content string) map[string]counterpartVersionResult {
	parse := func(path, body string) counterpartVersionResult {
		status, response := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)

		var parsed counterpartVersionResponse
		this.Require().NoError(json.Unmarshal([]byte(response), &parsed))

		return parsed.Results["orders"]
	}

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{"front", "production", []contractFragment{{"api.yaml", content}}})
	this.Require().NoError(err)

	return map[string]counterpartVersionResult{
		"can-i-deploy": parse("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`),
		"validate":     parse("/api/contracts/validate", string(validateBody)),
	}
}

func (this *IntegrationSuite) storedOrdersRowForCounterpartVersion() counterpartVersionRow {
	var row counterpartVersionRow
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT counterpart_participant_id, counterpart_version
		   FROM compatibility_check_results
		  WHERE counterpart_name = 'orders'`).Scan(&row.ParticipantID, &row.Version))

	return row
}

func (this *IntegrationSuite) ordersIDForCounterpartVersion() int64 {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = 'orders'`).Scan(&id))

	return id
}

func (this *IntegrationSuite) assertDeployedOrdersVersionKeptForCounterpartVersion(consumer, reason, endpoint string) {
	this.setupForCounterpartVersion()
	this.publishForCounterpartVersion("orders", "v1", counterpartVersionProviderYAML)
	this.deployForCounterpartVersion("orders", "v1", "production")
	this.publishForCounterpartVersion("orders", "v2", counterpartVersionProviderYAML)
	this.publishForCounterpartVersion("front", "v1", consumer)

	for name, result := range this.checkFrontForCounterpartVersion(consumer) {
		this.Require().Lenf(result.Endpoints[endpoint]["get"]["200"], 1, name)
		this.Equalf(reason, result.Endpoints[endpoint]["get"]["200"][0].Reason, name)
		this.Require().NotNilf(result.ParticipantVersion, name)
		this.Equalf("v1", *result.ParticipantVersion, name)
	}

	row := this.storedOrdersRowForCounterpartVersion()
	this.Require().NotNil(row.ParticipantID)
	this.Equal(this.ordersIDForCounterpartVersion(), *row.ParticipantID)
	this.Require().NotNil(row.Version)
	this.Equal("v1", *row.Version)
}

func (this *IntegrationSuite) TestCounterpartVersion_DeployedProviderWithoutTheEndpointKeepsItsVersion() {
	this.assertDeployedOrdersVersionKeptForCounterpartVersion(
		counterpartVersionMissingEndpointYAML, "provider_resource_not_found", "/pets")
}

func (this *IntegrationSuite) TestCounterpartVersion_DeployedProviderWithoutTheStatusKeepsItsVersion() {
	this.assertDeployedOrdersVersionKeptForCounterpartVersion(
		counterpartVersionMissingStatusYAML, "provider_resource_interaction_not_found", "/orders")
}

func (this *IntegrationSuite) assertOrdersNotDeployedForCounterpartVersion(participantID *int64) {
	for name, result := range this.checkFrontForCounterpartVersion(counterpartVersionMissingEndpointYAML) {
		leaves := result.Endpoints["/pets"]["get"]["200"]
		this.Require().Lenf(leaves, 1, name)
		this.Equalf("provider_resource_not_deployed_in_environment", leaves[0].Reason, name)
		this.Nilf(leaves[0].Details, name)
		this.Nilf(result.ParticipantVersion, name)
	}

	row := this.storedOrdersRowForCounterpartVersion()
	this.Equal(participantID, row.ParticipantID)
	this.Nil(row.Version)
}

func (this *IntegrationSuite) TestCounterpartVersion_ProviderDeployedOnlyElsewhereIsNotDeployedWithNoVersion() {
	this.setupForCounterpartVersion()
	this.publishForCounterpartVersion("orders", "v1", counterpartVersionProviderYAML)
	this.deployForCounterpartVersion("orders", "v1", "staging")
	this.publishForCounterpartVersion("front", "v1", counterpartVersionMissingEndpointYAML)

	ordersID := this.ordersIDForCounterpartVersion()
	this.assertOrdersNotDeployedForCounterpartVersion(&ordersID)
}

func (this *IntegrationSuite) TestCounterpartVersion_ProviderDeployedNowhereWithoutTheEndpointIsNotDeployedWithNoVersion() {
	this.setupForCounterpartVersion()
	this.publishForCounterpartVersion("orders", "v1", counterpartVersionProviderYAML)
	this.publishForCounterpartVersion("front", "v1", counterpartVersionMissingEndpointYAML)

	ordersID := this.ordersIDForCounterpartVersion()
	this.assertOrdersNotDeployedForCounterpartVersion(&ordersID)
}

func (this *IntegrationSuite) TestCounterpartVersion_RegisteredProviderNeverPublishedIsNotDeployedWithNoVersion() {
	this.setupForCounterpartVersion()
	this.publishForCounterpartVersion("front", "v1", counterpartVersionMissingEndpointYAML)

	ordersID := this.ordersIDForCounterpartVersion()
	this.assertOrdersNotDeployedForCounterpartVersion(&ordersID)
}

func (this *IntegrationSuite) TestCounterpartVersion_NameThatIsNotAParticipantIsNotDeployedWithNoParticipant() {
	this.mustPostForCounterpartVersion("/api/environments", `{"environment":"production"}`)
	this.mustPostForCounterpartVersion("/api/participants", `{"participant":"front"}`)
	this.publishForCounterpartVersion("front", "v1", counterpartVersionMissingEndpointYAML)

	this.assertOrdersNotDeployedForCounterpartVersion(nil)
}
