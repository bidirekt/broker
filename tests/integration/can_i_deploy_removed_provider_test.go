package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type removalBreakJSON struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type removalResultJSON struct {
	Deployable         bool                                                `json:"deployable"`
	ParticipantVersion *string                                             `json:"participantVersion"`
	Endpoints          map[string]map[string]map[string][]removalBreakJSON `json:"endpoints"`
}

type removalCanIDeployJSON struct {
	Deployable bool                         `json:"deployable"`
	Results    map[string]removalResultJSON `json:"results"`
}

type removalVerdictBreakJSON struct {
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Interaction string            `json:"interaction"`
	Reason      string            `json:"reason"`
	Details     map[string]string `json:"details"`
}

const removedProviderReason = "provider_resource_removed_but_still_consumed"

const removalCatalogV1Contract = `
{
  "provides": {
    "rest": {
      "/items": { "get": { "responses": { "200": "Item" } } },
      "/stock": { "get": { "responses": { "200": "Stock" } } }
    }
  },
  "schemas": {
    "Item":  { "type": "object", "properties": { "id":    { "type": "string"  } } },
    "Stock": { "type": "object", "properties": { "count": { "type": "integer" } } }
  }
}`

const removalCatalogV2Contract = `
{
  "provides": {
    "rest": {
      "/stock": { "get": { "responses": { "200": "Stock" } } }
    }
  },
  "schemas": {
    "Stock": { "type": "object", "properties": { "count": { "type": "integer" } } }
  }
}`

const removalCatalogV3Contract = `
{
  "provides": {
    "rest": {
      "/stock": { "get": { "responses": { "200": "Stock" } } }
    }
  },
  "schemas": {
    "Stock": {
      "type": "object",
      "properties": {
        "count":     { "type": "integer" },
        "updatedAt": { "type": "string"  }
      }
    }
  }
}`

const removalWebV1Contract = `
{
  "consumes": {
    "catalog": {
      "rest": {
        "/items": { "get": { "responses": { "200": "Item" } } },
        "/stock": { "get": { "responses": { "200": "Stock" } } }
      }
    }
  },
  "schemas": {
    "Item":  { "type": "object", "properties": { "id":    { "type": "string"  } } },
    "Stock": { "type": "object", "properties": { "count": { "type": "integer" } } }
  }
}`

const removalWebV2Contract = `
{
  "consumes": {
    "catalog": {
      "rest": {
        "/stock": { "get": { "responses": { "200": "Stock" } } }
      }
    }
  },
  "schemas": {
    "Stock": { "type": "object", "properties": { "count": { "type": "integer" } } }
  }
}`

const removalWebBreakingContract = `
{
  "consumes": {
    "catalog": {
      "rest": {
        "/items": { "get": { "responses": { "200": "Item" } } },
        "/stock": { "get": { "responses": { "200": "Stock" } } }
      }
    }
  },
  "schemas": {
    "Item":  { "type": "object", "properties": { "id": { "type": "string" } } },
    "Stock": {
      "type": "object",
      "properties": {
        "count": { "type": "integer" },
        "label": { "type": "string"  }
      }
    }
  }
}`

func (this *IntegrationSuite) mustPostForRemoval(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

// removalSetup publishes catalog v1 with /items and /stock, a consumer of both, and deploys
// both to production. The caller publishes the catalog version that drops /items.
func (this *IntegrationSuite) removalSetup(consumerContract string) {
	this.mustPostForRemoval("/api/environments", `{"environment":"production"}`)
	this.mustPostForRemoval("/api/participants", `{"participant":"catalog"}`)
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v1", contractFragment{"api.yaml", removalCatalogV1Contract}))
	this.mustPostForRemoval("/api/deployments", `{"participant":"catalog","version":"v1","environment":"production"}`)
	this.mustPostForRemoval("/api/participants", `{"participant":"web"}`)
	this.mustPostForRemoval("/api/contracts", this.publishBody("web", "v1", contractFragment{"api.yaml", consumerContract}))
	this.mustPostForRemoval("/api/deployments", `{"participant":"web","version":"v1","environment":"production"}`)
}

func (this *IntegrationSuite) canIDeployForRemoval(participant, version string) (removalCanIDeployJSON, string) {
	status, body := this.post(
		"/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`,
	)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var response removalCanIDeployJSON
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response, body
}

func (this *IntegrationSuite) verdictBreaksForRemoval() [][]removalVerdictBreakJSON {
	rows, err := this.Pool.Query(context.Background(), `SELECT breaks::text FROM compatibility_verdicts`)
	this.Require().NoError(err)
	defer rows.Close()

	verdicts := make([][]removalVerdictBreakJSON, 0)

	for rows.Next() {
		var raw string
		this.Require().NoError(rows.Scan(&raw))

		var breaks []removalVerdictBreakJSON
		this.Require().NoError(json.Unmarshal([]byte(raw), &breaks))

		verdicts = append(verdicts, breaks)
	}

	return verdicts
}

func (this *IntegrationSuite) TestRemovedProvider_BlocksWhileTheConsumerIsDeployed() {
	this.removalSetup(removalWebV1Contract)
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))

	response, body := this.canIDeployForRemoval("catalog", "v2")

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)

	web := response.Results["web"]
	this.False(web.Deployable)
	this.Require().NotNil(web.ParticipantVersion)
	this.Equal("v1", *web.ParticipantVersion)

	this.Require().Len(web.Endpoints, 1)
	breaks := web.Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal(removedProviderReason, breaks[0].Reason)
	this.Equal("provider", breaks[0].Role)
	this.Nil(breaks[0].Details)
	this.NotContains(body, `"details"`)
}

func (this *IntegrationSuite) TestRemovedProvider_IsAllowedOnceTheConsumerStopsConsuming() {
	this.removalSetup(removalWebV1Contract)
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))
	this.mustPostForRemoval("/api/contracts", this.publishBody("web", "v2", contractFragment{"api.yaml", removalWebV2Contract}))
	this.mustPostForRemoval("/api/deployments", `{"participant":"web","version":"v2","environment":"production"}`)

	response, _ := this.canIDeployForRemoval("catalog", "v2")

	this.True(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.True(response.Results["web"].Deployable)
	this.Empty(response.Results["web"].Endpoints)
}

func (this *IntegrationSuite) TestRemovedProvider_KeepsBlockingOnLaterVersions() {
	this.removalSetup(removalWebV1Contract)
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v3", contractFragment{"api.yaml", removalCatalogV3Contract}))

	response, _ := this.canIDeployForRemoval("catalog", "v3")

	this.False(response.Deployable)
	breaks := response.Results["web"].Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal(removedProviderReason, breaks[0].Reason)
}

func (this *IntegrationSuite) TestRemovedProvider_IsNeverStoredAsAVerdict() {
	this.removalSetup(removalWebBreakingContract)
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))

	// The removal and the property break land on the same counterpart in map order: repeating
	// the check on a clean slate exercises both arrival orders of the merge.
	for range 5 {
		_, err := this.Pool.Exec(context.Background(),
			`TRUNCATE compatibility_check_results, compatibility_checks, compatibility_verdicts`)
		this.Require().NoError(err)

		response, _ := this.canIDeployForRemoval("catalog", "v2")
		this.False(response.Deployable)

		reasons := make([]string, 0)
		for _, methods := range response.Results["web"].Endpoints {
			for _, interactions := range methods {
				for _, breaks := range interactions {
					for _, breakChange := range breaks {
						reasons = append(reasons, breakChange.Reason)
					}
				}
			}
		}
		this.ElementsMatch([]string{removedProviderReason, "property_missing_in_provider"}, reasons)

		verdicts := this.verdictBreaksForRemoval()
		this.Require().Len(verdicts, 1)
		this.Require().Len(verdicts[0], 1)
		this.Equal("property_missing_in_provider", verdicts[0][0].Reason)
		this.Equal("/stock", verdicts[0][0].Endpoint)
	}
}

func (this *IntegrationSuite) TestRemovedProvider_ConsumerCheckIgnoresItsOwnRemovedConsumption() {
	this.removalSetup(removalWebV1Contract)
	this.mustPostForRemoval("/api/contracts", this.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))
	this.mustPostForRemoval("/api/deployments", `{"participant":"catalog","version":"v2","environment":"production"}`)
	this.mustPostForRemoval("/api/contracts", this.publishBody("web", "v2", contractFragment{"api.yaml", removalWebV2Contract}))

	response, _ := this.canIDeployForRemoval("web", "v2")

	this.True(response.Deployable)
	this.Require().Len(response.Results, 1)
	this.True(response.Results["catalog"].Deployable)
	this.Empty(response.Results["catalog"].Endpoints)
}
