package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type removalBreakJSON struct {
	Reason  string            `json:"reason"`
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

func (s *IntegrationSuite) mustPostForRemoval(path, body string) {
	status, response := s.post(path, body)
	s.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

// removalSetup publishes catalog v1 with /items and /stock, a consumer of both, and deploys
// both to production. The caller publishes the catalog version that drops /items.
func (s *IntegrationSuite) removalSetup(consumerContract string) {
	s.mustPostForRemoval("/api/environments", `{"environment":"production"}`)
	s.mustPostForRemoval("/api/participants", `{"participant":"catalog"}`)
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v1", contractFragment{"api.yaml", removalCatalogV1Contract}))
	s.mustPostForRemoval("/api/deployments", `{"participant":"catalog","version":"v1","environment":"production"}`)
	s.mustPostForRemoval("/api/participants", `{"participant":"web"}`)
	s.mustPostForRemoval("/api/contracts", s.publishBody("web", "v1", contractFragment{"api.yaml", consumerContract}))
	s.mustPostForRemoval("/api/deployments", `{"participant":"web","version":"v1","environment":"production"}`)
}

func (s *IntegrationSuite) canIDeployForRemoval(participant, version string) (removalCanIDeployJSON, string) {
	status, body := s.post(
		"/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`,
	)
	s.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var response removalCanIDeployJSON
	s.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response, body
}

func (s *IntegrationSuite) verdictBreaksForRemoval() [][]removalVerdictBreakJSON {
	rows, err := s.Pool.Query(context.Background(), `SELECT breaks::text FROM compatibility_verdicts`)
	s.Require().NoError(err)
	defer rows.Close()

	verdicts := make([][]removalVerdictBreakJSON, 0)

	for rows.Next() {
		var raw string
		s.Require().NoError(rows.Scan(&raw))

		var breaks []removalVerdictBreakJSON
		s.Require().NoError(json.Unmarshal([]byte(raw), &breaks))

		verdicts = append(verdicts, breaks)
	}

	return verdicts
}

func (s *IntegrationSuite) TestRemovedProvider_BlocksWhileTheConsumerIsDeployed() {
	s.removalSetup(removalWebV1Contract)
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))

	response, body := s.canIDeployForRemoval("catalog", "v2")

	s.False(response.Deployable)
	s.Require().Len(response.Results, 1)

	web := response.Results["web"]
	s.False(web.Deployable)
	s.Require().NotNil(web.ParticipantVersion)
	s.Equal("v1", *web.ParticipantVersion)

	s.Require().Len(web.Endpoints, 1)
	breaks := web.Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal(removedProviderReason, breaks[0].Reason)
	s.Nil(breaks[0].Details)
	s.NotContains(body, `"details"`)
}

func (s *IntegrationSuite) TestRemovedProvider_IsAllowedOnceTheConsumerStopsConsuming() {
	s.removalSetup(removalWebV1Contract)
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))
	s.mustPostForRemoval("/api/contracts", s.publishBody("web", "v2", contractFragment{"api.yaml", removalWebV2Contract}))
	s.mustPostForRemoval("/api/deployments", `{"participant":"web","version":"v2","environment":"production"}`)

	response, _ := s.canIDeployForRemoval("catalog", "v2")

	s.True(response.Deployable)
	s.Require().Len(response.Results, 1)
	s.True(response.Results["web"].Deployable)
	s.Empty(response.Results["web"].Endpoints)
}

func (s *IntegrationSuite) TestRemovedProvider_KeepsBlockingOnLaterVersions() {
	s.removalSetup(removalWebV1Contract)
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v3", contractFragment{"api.yaml", removalCatalogV3Contract}))

	response, _ := s.canIDeployForRemoval("catalog", "v3")

	s.False(response.Deployable)
	breaks := response.Results["web"].Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal(removedProviderReason, breaks[0].Reason)
}

func (s *IntegrationSuite) TestRemovedProvider_IsNeverStoredAsAVerdict() {
	s.removalSetup(removalWebBreakingContract)
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))

	// The removal and the property break land on the same counterpart in map order: repeating
	// the check on a clean slate exercises both arrival orders of the merge.
	for range 5 {
		_, err := s.Pool.Exec(context.Background(),
			`TRUNCATE compatibility_check_results, compatibility_checks, compatibility_verdicts`)
		s.Require().NoError(err)

		response, _ := s.canIDeployForRemoval("catalog", "v2")
		s.False(response.Deployable)

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
		s.ElementsMatch([]string{removedProviderReason, "property_missing_in_provider"}, reasons)

		verdicts := s.verdictBreaksForRemoval()
		s.Require().Len(verdicts, 1)
		s.Require().Len(verdicts[0], 1)
		s.Equal("property_missing_in_provider", verdicts[0][0].Reason)
		s.Equal("/stock", verdicts[0][0].Endpoint)
	}
}

func (s *IntegrationSuite) TestRemovedProvider_ConsumerCheckIgnoresItsOwnRemovedConsumption() {
	s.removalSetup(removalWebV1Contract)
	s.mustPostForRemoval("/api/contracts", s.publishBody("catalog", "v2", contractFragment{"api.yaml", removalCatalogV2Contract}))
	s.mustPostForRemoval("/api/deployments", `{"participant":"catalog","version":"v2","environment":"production"}`)
	s.mustPostForRemoval("/api/contracts", s.publishBody("web", "v2", contractFragment{"api.yaml", removalWebV2Contract}))

	response, _ := s.canIDeployForRemoval("web", "v2")

	s.True(response.Deployable)
	s.Require().Len(response.Results, 1)
	s.True(response.Results["catalog"].Deployable)
	s.Empty(response.Results["catalog"].Endpoints)
}
