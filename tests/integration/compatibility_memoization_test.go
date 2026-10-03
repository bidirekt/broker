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

func (s *IntegrationSuite) mustPostForMemoization(path, body string) {
	status, response := s.post(path, body)
	s.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (s *IntegrationSuite) canIDeployForMemoization(participant, version string) (string, memoizationResponse) {
	status, body := s.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	s.Require().Equalf(http.StatusOK, status, "can-i-deploy %s@%s: %s", participant, version, body)

	var parsed memoizationResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return body, parsed
}

func (s *IntegrationSuite) storedVerdictBreaksForMemoization() string {
	var breaks string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT breaks::text FROM compatibility_verdicts`).Scan(&breaks))
	return breaks
}

func (s *IntegrationSuite) storedVerdictForMemoization() []memoizationStoredBreak {
	var breaks []byte
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT breaks FROM compatibility_verdicts`).Scan(&breaks))

	var stored []memoizationStoredBreak
	s.Require().NoError(json.Unmarshal(breaks, &stored))

	return stored
}

func (s *IntegrationSuite) rewriteStoredVerdictForMemoization(breaks string) {
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE compatibility_verdicts SET breaks = $1`, breaks)
	s.Require().NoError(err)
}

// The pair is deployed on both sides, so either participant can be the one checking — and both
// directions have to land on the single canonical verdict row.
func (s *IntegrationSuite) TestMemoization_IdenticalChecksReplayTheStoredVerdict() {
	s.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	s.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	s.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	s.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))
	s.mustPostForMemoization("/api/deployments",
		`{"participant":"cart","version":"v1","environment":"production"}`)

	firstBody, first := s.canIDeployForMemoization("cart", "v1")
	s.False(first.Deployable)

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
	s.Equal([]memoizationBreak{expectedBreak},
		first.Results["orders"].Endpoints["/orders"]["get"]["200"])

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(1, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	secondBody, _ := s.canIDeployForMemoization("cart", "v1")
	s.Equal(firstBody, secondBody)

	s.Equal(2, s.countRows("compatibility_checks"))
	s.Equal(2, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	// Rewriting the fact to something the live diff cannot produce shows the reversed direction
	// resolves to the very same canonical row instead of recomputing its own answer.
	s.rewriteStoredVerdictForMemoization(memoizationFabricatedBreaks)

	_, fromTheOtherSide := s.canIDeployForMemoization("orders", "v1")
	s.False(fromTheOtherSide.Deployable)
	s.Equal([]memoizationBreak{{
		Reason: "property_missing_in_consumer",
		Role:   "provider",
		Details: map[string]string{
			"property":     "$.fabricated",
			"consumerName": "cart",
			"providerName": "orders",
			"propertyType": "boolean",
		},
	}}, fromTheOtherSide.Results["cart"].Endpoints["/orders"]["get"]["200"])

	s.Equal(3, s.countRows("compatibility_checks"))
	s.Equal(3, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	var contractIDOne, contractIDTwo int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT contract_id_one, contract_id_two FROM compatibility_verdicts`).
		Scan(&contractIDOne, &contractIDTwo))
	s.Less(contractIDOne, contractIDTwo)

	var pairedResults int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM compatibility_check_results
		  WHERE verdict_contract_id_one = $1 AND verdict_contract_id_two = $2`,
		contractIDOne, contractIDTwo).Scan(&pairedResults))
	s.Equal(3, pairedResults)
}

// Republishing the same content under another version name is an alias: same snapshot, same
// pair, so the stored verdict answers for it.
func (s *IntegrationSuite) TestMemoization_AliasedVersionHitsTheStoredVerdict() {
	s.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	s.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	s.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	s.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))

	_, first := s.canIDeployForMemoization("cart", "v1")
	s.False(first.Deployable)
	s.Equal(1, s.countRows("compatibility_verdicts"))

	s.mustPostForMemoization("/api/contracts",
		s.publishBody("cart", "a1b2c3d", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))

	_, aliased := s.canIDeployForMemoization("cart", "a1b2c3d")

	s.Equal(first.Results, aliased.Results)
	s.Equal(first.Deployable, aliased.Deployable)

	s.Equal(2, s.countRows("compatibility_checks"))
	s.Equal(2, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))
}

// A cached pair does not silence the environment checks: not_found and not_deployed depend on
// deployments, never on the verdict, so they are evaluated live on every call.
func (s *IntegrationSuite) TestMemoization_CachedVerdictKeepsLiveEnvironmentBreaks() {
	s.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	s.mustPostForMemoization("/api/participants", `{"participant":"billing"}`)
	s.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	s.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	s.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("billing", "v1", contractFragment{"api.yaml", memoizationBillingProviderContract}))
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationMixedConsumerContract}))

	firstBody, first := s.canIDeployForMemoization("cart", "v1")
	s.False(first.Deployable)

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
	s.Equal([]memoizationBreak{expectedBreak},
		first.Results["orders"].Endpoints["/orders"]["get"]["200"])
	s.Equal("provider_resource_not_found",
		first.Results["orders"].Endpoints["/ghosts"]["get"]["200"][0].Reason)
	s.Equal("provider_resource_not_deployed_in_environment",
		first.Results["billing"].Endpoints["/invoices"]["get"]["200"][0].Reason)

	s.Equal(1, s.countRows("compatibility_verdicts"))
	s.Equal([]memoizationStoredBreak{{
		Endpoint:    "/orders",
		Method:      "get",
		Interaction: "200",
		Reason:      "property_missing_in_provider",
		Details:     expectedBreak.Details,
	}}, s.storedVerdictForMemoization())

	secondBody, second := s.canIDeployForMemoization("cart", "v1")
	s.Equal(firstBody, secondBody)
	s.Equal([]memoizationBreak{expectedBreak},
		second.Results["orders"].Endpoints["/orders"]["get"]["200"])

	s.Equal(2, s.countRows("compatibility_checks"))
	s.Equal(4, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	var pairedResults int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM compatibility_check_results
		  WHERE verdict_contract_id_one IS NOT NULL`).Scan(&pairedResults))
	s.Equal(2, pairedResults)
}

// The stored verdict is rewritten to something the live diff would never produce: whatever the
// response carries afterwards can only have come from storage, not from checkResources.
func (s *IntegrationSuite) TestMemoization_HitOnACompatiblePairSkipsTheDiff() {
	s.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	s.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	s.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	s.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationCompatibleConsumerContract}))

	_, first := s.canIDeployForMemoization("cart", "v1")
	s.True(first.Deployable)
	s.Empty(first.Results["orders"].Endpoints)

	s.Equal(1, s.countRows("compatibility_verdicts"))
	s.Equal("[]", s.storedVerdictBreaksForMemoization())

	s.rewriteStoredVerdictForMemoization(memoizationFabricatedBreaks)

	_, second := s.canIDeployForMemoization("cart", "v1")
	s.False(second.Deployable)
	s.Equal([]memoizationBreak{{
		Reason: "property_missing_in_consumer",
		Role:   "consumer",
		Details: map[string]string{
			"property":     "$.fabricated",
			"consumerName": "cart",
			"providerName": "orders",
			"propertyType": "boolean",
		},
	}}, second.Results["orders"].Endpoints["/orders"]["get"]["200"])
	s.False(second.Results["orders"].Deployable)
	s.Require().NotNil(second.Results["orders"].ParticipantVersion)
	s.Equal("v1", *second.Results["orders"].ParticipantVersion)

	s.Equal(1, s.countRows("compatibility_verdicts"))

	var deployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT deployable FROM compatibility_checks ORDER BY id DESC LIMIT 1`).Scan(&deployable))
	s.False(deployable)
}

func (s *IntegrationSuite) TestMemoization_HitOnAnIncompatiblePairSkipsTheDiff() {
	s.mustPostForMemoization("/api/participants", `{"participant":"orders"}`)
	s.mustPostForMemoization("/api/participants", `{"participant":"cart"}`)
	s.mustPostForMemoization("/api/environments", `{"environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("orders", "v1", contractFragment{"api.yaml", memoizationOrdersProviderContract}))
	s.mustPostForMemoization("/api/deployments",
		`{"participant":"orders","version":"v1","environment":"production"}`)
	s.mustPostForMemoization("/api/contracts",
		s.publishBody("cart", "v1", contractFragment{"api.yaml", memoizationOrdersConsumerContract}))

	_, first := s.canIDeployForMemoization("cart", "v1")
	s.False(first.Deployable)
	s.Equal(1, s.countRows("compatibility_verdicts"))

	s.rewriteStoredVerdictForMemoization(`[]`)

	_, second := s.canIDeployForMemoization("cart", "v1")
	s.True(second.Deployable)
	s.True(second.Results["orders"].Deployable)
	s.Empty(second.Results["orders"].Endpoints)

	s.Equal(1, s.countRows("compatibility_verdicts"))
}
