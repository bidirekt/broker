package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type memoizationBreak struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type memoizationStoredBreak struct {
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Interaction string            `json:"interaction"`
	Reason      string            `json:"reason"`
	Details     map[string]string `json:"details"`
}

type memoizationResult struct {
	Deployable         bool                                                `json:"deployable"`
	ParticipantVersion *string                                             `json:"participantVersion"`
	Endpoints          map[string]map[string]map[string][]memoizationBreak `json:"endpoints"`
}

type memoizationResponse struct {
	Deployable bool                         `json:"deployable"`
	Results    map[string]memoizationResult `json:"results"`
}

const memoizationOrdersProviderContract = `
{
  "provides": { "rest": { "/orders": { "get": { "responses": { "200": "Order" } } } } },
  "schemas": { "Order": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const memoizationOrdersConsumerContract = `
{
  "consumes": { "orders": { "rest": { "/orders": { "get": { "responses": { "200": "Order" } } } } } },
  "schemas": {
    "Order": {
      "type": "object",
      "properties": {
        "id":    { "type": "string" },
        "total": { "type": "integer" }
      }
    }
  }
}`

const memoizationCompatibleConsumerContract = `
{
  "consumes": { "orders": { "rest": { "/orders": { "get": { "responses": { "200": "Order" } } } } } },
  "schemas": { "Order": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const memoizationMixedConsumerContract = `
{
  "consumes": {
    "orders": {
      "rest": {
        "/orders": { "get": { "responses": { "200": "Order" } } },
        "/ghosts": { "get": { "responses": { "200": "Order" } } }
      }
    },
    "billing": {
      "rest": { "/invoices": { "get": { "responses": { "200": "Invoice" } } } }
    }
  },
  "schemas": {
    "Order": {
      "type": "object",
      "properties": {
        "id":    { "type": "string" },
        "total": { "type": "integer" }
      }
    },
    "Invoice": { "type": "object", "properties": { "id": { "type": "string" } } }
  }
}`

const memoizationBillingProviderContract = `
{
  "provides": { "rest": { "/invoices": { "get": { "responses": { "200": "Invoice" } } } } },
  "schemas": { "Invoice": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const memoizationFabricatedBreaks = `
[{ "endpoint": "/orders", "method": "get", "interaction": "200",
   "reason": "property_missing_in_consumer",
   "details": { "property": "$.fabricated", "consumerName": "cart",
                "providerName": "orders", "propertyType": "boolean" } }]`

func (this *IntegrationSuite) mustPostForMemoization(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForMemoization(participant, version string) (string, memoizationResponse) {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s@%s: %s", participant, version, body)

	var parsed memoizationResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return body, parsed
}

func (this *IntegrationSuite) storedVerdictBreaksForMemoization() string {
	var breaks string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT breaks::text FROM compatibility_verdicts`).Scan(&breaks))
	return breaks
}

func (this *IntegrationSuite) storedVerdictForMemoization() []memoizationStoredBreak {
	var breaks []byte
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT breaks FROM compatibility_verdicts`).Scan(&breaks))

	var stored []memoizationStoredBreak
	this.Require().NoError(json.Unmarshal(breaks, &stored))

	return stored
}

func (this *IntegrationSuite) rewriteStoredVerdictForMemoization(breaks string) {
	_, err := this.Pool.Exec(context.Background(),
		`UPDATE compatibility_verdicts SET breaks = $1`, breaks)
	this.Require().NoError(err)
}

// The pair is deployed on both sides, so either participant can be the one checking — and both
// directions have to land on the single canonical verdict row.
func (this *IntegrationSuite) TestMemoization_IdenticalChecksReplayTheStoredVerdict() {
	this.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	this.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	this.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	this.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))
	this.mustPostForMemoization("/api/deployments",
		`{"participant":"cart","version":"v1","environment":"production"}`)

	firstBody, first := this.canIDeployForMemoization("cart", "v1")
	this.False(first.Deployable)

	expectedBreak := memoizationBreak{
		Reason: "property_missing_in_provider",
		Role:   "consumer",
		Details: map[string]string{
			"property":     "$.total",
			"consumerName": "cart",
			"providerName": "orders",
			"propertyType": "integer",
		},
	}
	this.Equal([]memoizationBreak{expectedBreak},
		first.Results["orders"].Endpoints["/orders"]["get"]["200"])

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	secondBody, _ := this.canIDeployForMemoization("cart", "v1")
	this.Equal(firstBody, secondBody)

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(2, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	// Rewriting the fact to something the live diff cannot produce shows the reversed direction
	// resolves to the very same canonical row instead of recomputing its own answer.
	this.rewriteStoredVerdictForMemoization(memoizationFabricatedBreaks)

	_, fromTheOtherSide := this.canIDeployForMemoization("orders", "v1")
	this.False(fromTheOtherSide.Deployable)
	this.Equal([]memoizationBreak{{
		Reason: "property_missing_in_consumer",
		Role:   "provider",
		Details: map[string]string{
			"property":     "$.fabricated",
			"consumerName": "cart",
			"providerName": "orders",
			"propertyType": "boolean",
		},
	}}, fromTheOtherSide.Results["cart"].Endpoints["/orders"]["get"]["200"])

	this.Equal(3, this.countRows("compatibility_checks"))
	this.Equal(3, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	var contractIDOne, contractIDTwo int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id_one, contract_id_two FROM compatibility_verdicts`).
		Scan(&contractIDOne, &contractIDTwo))
	this.Less(contractIDOne, contractIDTwo)

	var pairedResults int
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM compatibility_check_results
		  WHERE verdict_contract_id_one = $1 AND verdict_contract_id_two = $2`,
		contractIDOne, contractIDTwo).Scan(&pairedResults))
	this.Equal(3, pairedResults)
}

// Republishing the same content under another version name is an alias: same snapshot, same
// pair, so the stored verdict answers for it.
func (this *IntegrationSuite) TestMemoization_AliasedVersionHitsTheStoredVerdict() {
	this.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	this.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	this.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	this.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))

	_, first := this.canIDeployForMemoization("cart", "v1")
	this.False(first.Deployable)
	this.Equal(1, this.countRows("compatibility_verdicts"))

	this.mustPostForMemoization("/api/contracts",
		this.publishBody("cart", "a1b2c3d", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))

	_, aliased := this.canIDeployForMemoization("cart", "a1b2c3d")

	this.Equal(first.Results, aliased.Results)
	this.Equal(first.Deployable, aliased.Deployable)

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(2, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))
}

// A cached pair does not silence the environment checks: not_found and not_deployed depend on
// deployments, never on the verdict, so they are evaluated live on every call.
func (this *IntegrationSuite) TestMemoization_CachedVerdictKeepsLiveEnvironmentBreaks() {
	this.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	this.mustPostForMemoization("/api/participants", `{"participant":"billing"}`)
	this.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	this.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	this.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("billing", "v1", contractFragment{"api.yaml", memoizationBillingProviderContract}))
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationMixedConsumerContract}))

	firstBody, first := this.canIDeployForMemoization("cart", "v1")
	this.False(first.Deployable)

	expectedBreak := memoizationBreak{
		Reason: "property_missing_in_provider",
		Role:   "consumer",
		Details: map[string]string{
			"property":     "$.total",
			"consumerName": "cart",
			"providerName": "orders",
			"propertyType": "integer",
		},
	}
	this.Equal([]memoizationBreak{expectedBreak},
		first.Results["orders"].Endpoints["/orders"]["get"]["200"])
	this.Equal("provider_resource_not_found",
		first.Results["orders"].Endpoints["/ghosts"]["get"]["200"][0].Reason)
	this.Equal("provider_resource_not_deployed_in_environment",
		first.Results["billing"].Endpoints["/invoices"]["get"]["200"][0].Reason)

	this.Equal(1, this.countRows("compatibility_verdicts"))
	this.Equal([]memoizationStoredBreak{{
		Endpoint:    "/orders",
		Method:      "get",
		Interaction: "200",
		Reason:      "property_missing_in_provider",
		Details:     expectedBreak.Details,
	}}, this.storedVerdictForMemoization())

	secondBody, second := this.canIDeployForMemoization("cart", "v1")
	this.Equal(firstBody, secondBody)
	this.Equal([]memoizationBreak{expectedBreak},
		second.Results["orders"].Endpoints["/orders"]["get"]["200"])

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(4, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	var pairedResults int
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM compatibility_check_results
		  WHERE verdict_contract_id_one IS NOT NULL`).Scan(&pairedResults))
	this.Equal(2, pairedResults)
}

// The stored verdict is rewritten to something the live diff would never produce: whatever the
// response carries afterwards can only have come from storage, not from checkResources.
func (this *IntegrationSuite) TestMemoization_HitOnACompatiblePairSkipsTheDiff() {
	this.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	this.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	this.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	this.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationCompatibleConsumerContract}))

	_, first := this.canIDeployForMemoization("cart", "v1")
	this.True(first.Deployable)
	this.Empty(first.Results["orders"].Endpoints)

	this.Equal(1, this.countRows("compatibility_verdicts"))
	this.Equal("[]", this.storedVerdictBreaksForMemoization())

	this.rewriteStoredVerdictForMemoization(memoizationFabricatedBreaks)

	_, second := this.canIDeployForMemoization("cart", "v1")
	this.False(second.Deployable)
	this.Equal([]memoizationBreak{{
		Reason: "property_missing_in_consumer",
		Role:   "consumer",
		Details: map[string]string{
			"property":     "$.fabricated",
			"consumerName": "cart",
			"providerName": "orders",
			"propertyType": "boolean",
		},
	}}, second.Results["orders"].Endpoints["/orders"]["get"]["200"])
	this.False(second.Results["orders"].Deployable)
	this.Require().NotNil(second.Results["orders"].ParticipantVersion)
	this.Equal("v1", *second.Results["orders"].ParticipantVersion)

	this.Equal(1, this.countRows("compatibility_verdicts"))

	var deployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT deployable FROM compatibility_checks ORDER BY id DESC LIMIT 1`).Scan(&deployable))
	this.False(deployable)
}

func (this *IntegrationSuite) TestMemoization_HitOnAnIncompatiblePairSkipsTheDiff() {
	this.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	this.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	this.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	this.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	this.mustPostForMemoization("/api/contracts",
		this.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))

	_, first := this.canIDeployForMemoization("cart", "v1")
	this.False(first.Deployable)
	this.Equal(1, this.countRows("compatibility_verdicts"))

	this.rewriteStoredVerdictForMemoization(`[]`)

	_, second := this.canIDeployForMemoization("cart", "v1")
	this.True(second.Deployable)
	this.True(second.Results["orders"].Deployable)
	this.Empty(second.Results["orders"].Endpoints)

	this.Equal(1, this.countRows("compatibility_verdicts"))
}
