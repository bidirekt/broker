package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type validateBreakJSON struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type validateResultJSON struct {
	Deployable         bool                                                 `json:"deployable"`
	ParticipantVersion *string                                              `json:"participantVersion"`
	Endpoints          map[string]map[string]map[string][]validateBreakJSON `json:"endpoints"`
}

type validateResponseJSON struct {
	Message     string                        `json:"message"`
	Participant string                        `json:"participant"`
	Environment string                        `json:"environment"`
	Deployable  bool                          `json:"deployable"`
	Results     map[string]validateResultJSON `json:"results"`
}

type validateFile struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

var validateTables = []string{
	"participants",
	"environments",
	"contracts",
	"contract_versions",
	"resources",
	"resource_versions",
	"properties",
	"property_versions",
	"deployments",
	"compatibility_verdicts",
	"compatibility_checks",
	"compatibility_check_results",
}

const validateCatalogItemsAndStockYAML = `provides:
  rest:
    /items:
      get:
        responses:
          200: Item
    /stock:
      get:
        responses:
          200: Stock
schemas:
  Item:
    type: object
    properties:
      id:
        type: string
  Stock:
    type: object
    properties:
      count:
        type: integer
`

const validateCatalogStockYAML = `provides:
  rest:
    /stock:
      get:
        responses:
          200: Stock
schemas:
  Stock:
    type: object
    properties:
      count:
        type: integer
`

const validateCatalogIntegerItemsYAML = `provides:
  rest:
    /items:
      get:
        responses:
          200: Item
schemas:
  Item:
    type: object
    properties:
      id:
        type: integer
`

const validateWebItemsYAML = `consumes:
  catalog:
    rest:
      /items:
        get:
          responses:
            200: Item
schemas:
  Item:
    type: object
    properties:
      id:
        type: string
`

const validateWebIntegerItemsYAML = `consumes:
  catalog:
    rest:
      /items:
        get:
          responses:
            200: Item
schemas:
  Item:
    type: object
    properties:
      id:
        type: integer
`

const validateUnknownKeyYAML = `provides:
  message:
    items.created:
      payload: Item
`

const validateUnresolvedSchemaYAML = `provides:
  rest:
    /items:
      get:
        responses:
          200: Missing
`

func (s *IntegrationSuite) mustPostForValidate(path, body string) {
	status, response := s.post(path, body)
	s.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (s *IntegrationSuite) validateBody(participant, environment string, files ...validateFile) string {
	body, err := json.Marshal(struct {
		Participant string         `json:"participant"`
		Environment string         `json:"environment"`
		Contracts   []validateFile `json:"contracts"`
	}{
		Participant: participant,
		Environment: environment,
		Contracts:   files,
	})
	s.Require().NoError(err)

	return string(body)
}

func (s *IntegrationSuite) validateOK(participant string, files ...validateFile) (validateResponseJSON, string) {
	status, body := s.post("/api/contracts/validate", s.validateBody(participant, "production", files...))
	s.Require().Equalf(http.StatusOK, status, "validate %s: %s", participant, body)

	var response validateResponseJSON
	s.Require().NoError(json.Unmarshal([]byte(body), &response))
	s.Equal("contract validated successfully", response.Message)
	s.Equal(participant, response.Participant)
	s.Equal("production", response.Environment)

	return response, body
}

func (s *IntegrationSuite) publishAndDeployForValidate(participant, version, content string) {
	s.mustPostForValidate("/api/contracts", s.publishBody(participant, version, contractFragment{"api.yaml", content}))
	s.mustPostForValidate("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (s *IntegrationSuite) setupForValidate(participants ...string) {
	s.mustPostForValidate("/api/environments", `{"environment":"production"}`)
	for _, participant := range participants {
		s.mustPostForValidate("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (s *IntegrationSuite) rowCountsForValidate() map[string]int {
	counts := make(map[string]int, len(validateTables))
	for _, table := range validateTables {
		counts[table] = s.countRows(table)
	}

	return counts
}

func (s *IntegrationSuite) TestValidateContract_InvalidInput() {
	s.setupForValidate("catalog")

	file := validateFile{"api.yaml", validateCatalogStockYAML}
	cases := map[string]string{
		"unparseable body":    `{"participant":`,
		"empty participant":   s.validateBody(" ", "production", file),
		"empty environment":   s.validateBody("catalog", "", file),
		"empty contracts":     s.validateBody("catalog", "production"),
		"blank source":        s.validateBody("catalog", "production", validateFile{"  ", validateCatalogStockYAML}),
		"blank unknown names": s.validateBody("ghost", "nowhere", validateFile{"", ""}, validateFile{"notes.txt", "x"}),
	}

	for name, body := range cases {
		status, response := s.post("/api/contracts/validate", body)
		s.Equal(http.StatusBadRequest, status, name)
		s.JSONEq(`{"message":"contract invalid input"}`, response, name)
	}
}

func (s *IntegrationSuite) TestValidateContract_DecodeErrorComesBeforeParticipantNotFound() {
	status, body := s.post("/api/contracts/validate",
		s.validateBody("ghost", "nowhere", validateFile{"notes.txt", validateCatalogStockYAML}))
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"unsupported contract file: notes.txt (expected .yaml or .yml)"}`, body)

	status, body = s.post("/api/contracts/validate",
		s.validateBody("ghost", "nowhere", validateFile{"broken.yaml", "provides: {"}))
	s.Equal(http.StatusBadRequest, status)
	s.Contains(body, `"message":"malformed contract file: broken.yaml:`)
}

func (s *IntegrationSuite) TestValidateContract_ShapeViolationComesBeforeParticipantNotFound() {
	status, body := s.post("/api/contracts/validate",
		s.validateBody("ghost", "nowhere", validateFile{"api.yaml", validateUnknownKeyYAML}))

	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"key.unknown","path":"provides;message","source":"api.yaml","details":{"key":"message"}}`+
		`]}`, body)
}

func (s *IntegrationSuite) TestValidateContract_ParticipantNotFoundComesBeforeEnvironmentNotFound() {
	status, body := s.post("/api/contracts/validate",
		s.validateBody("ghost", "nowhere", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))

	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"participant not found"}`, body)
}

func (s *IntegrationSuite) TestValidateContract_EnvironmentNotFoundComesBeforeRuleViolation() {
	s.setupForValidate("catalog")

	status, body := s.post("/api/contracts/validate",
		s.validateBody("catalog", "nowhere", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))

	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"environment not found"}`, body)
}

func (s *IntegrationSuite) TestValidateContract_RuleViolation() {
	s.setupForValidate("catalog")

	status, body := s.post("/api/contracts/validate",
		s.validateBody(" catalog ", " production ", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))
	s.Equal(http.StatusBadRequest, status)

	var response struct {
		Message    string `json:"message"`
		Violations []struct {
			Code   string `json:"code"`
			Source string `json:"source"`
		} `json:"violations"`
	}
	s.Require().NoError(json.Unmarshal([]byte(body), &response))
	s.Equal("contract validation failed", response.Message)
	s.Require().Len(response.Violations, 1)
	s.Equal("schema.unresolved_name", response.Violations[0].Code)
	s.Equal("api.yaml", response.Violations[0].Source)
}

func (s *IntegrationSuite) TestValidateContract_Compatible() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)

	response, _ := s.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	s.True(response.Deployable)
	s.Require().Len(response.Results, 1)
	catalog := response.Results["catalog"]
	s.True(catalog.Deployable)
	s.Empty(catalog.Endpoints)
	s.Require().NotNil(catalog.ParticipantVersion)
	s.Equal("v1", *catalog.ParticipantVersion)
}

func (s *IntegrationSuite) TestValidateContract_IncompatibleWithDeployedProvider() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)

	response, _ := s.validateOK("web", validateFile{"api.yaml", validateWebIntegerItemsYAML})

	s.False(response.Deployable)
	s.Require().Len(response.Results, 1)
	catalog := response.Results["catalog"]
	s.False(catalog.Deployable)

	breaks := catalog.Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal("property_type_mismatch", breaks[0].Reason)
}

func (s *IntegrationSuite) TestValidateContract_ConsumerOfNeverPublishedProvider() {
	s.setupForValidate("catalog", "web")

	response, _ := s.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	s.False(response.Deployable)
	breaks := response.Results["catalog"].Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal("provider_resource_not_found", breaks[0].Reason)
}

func (s *IntegrationSuite) TestValidateContract_ConsumerOfProviderNotDeployedInEnvironment() {
	s.setupForValidate("catalog", "web")
	s.mustPostForValidate("/api/contracts",
		s.publishBody("catalog", "v1", contractFragment{"api.yaml", validateCatalogItemsAndStockYAML}))

	response, _ := s.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	s.False(response.Deployable)
	breaks := response.Results["catalog"].Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal("provider_resource_not_deployed_in_environment", breaks[0].Reason)
}

func (s *IntegrationSuite) TestValidateContract_IgnoresPublishedButNotDeployedVersions() {
	s.setupForValidate("catalog", "web", "billing")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	s.mustPostForValidate("/api/contracts",
		s.publishBody("catalog", "v2", contractFragment{"api.yaml", validateCatalogIntegerItemsYAML}))
	s.mustPostForValidate("/api/contracts",
		s.publishBody("billing", "v1", contractFragment{"api.yaml", validateWebIntegerItemsYAML}))

	consumer, _ := s.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})
	s.True(consumer.Deployable)
	s.Require().NotNil(consumer.Results["catalog"].ParticipantVersion)
	s.Equal("v1", *consumer.Results["catalog"].ParticipantVersion)

	provider, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogItemsAndStockYAML})
	s.True(provider.Deployable)
	s.NotContains(provider.Results, "billing")
}

func (s *IntegrationSuite) TestValidateContract_WithoutConsumesInEnvironmentWithoutConsumers() {
	s.setupForValidate("catalog")

	response, body := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogItemsAndStockYAML})

	s.True(response.Deployable)
	s.Empty(response.Results)
	s.JSONEq(`{"message":"contract validated successfully","participant":"catalog","environment":"production","deployable":true,"results":{}}`, body)
}

func (s *IntegrationSuite) TestValidateContract_StoresNothing() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	s.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	before := s.rowCountsForValidate()

	compatible, _ := s.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})
	s.True(compatible.Deployable)
	s.Equal(before, s.rowCountsForValidate())

	incompatible, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogIntegerItemsYAML})
	s.False(incompatible.Deployable)
	s.Equal(before, s.rowCountsForValidate())

	status, _ := s.post("/api/contracts/validate",
		s.validateBody("catalog", "production", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))
	s.Equal(http.StatusBadRequest, status)
	s.Equal(before, s.rowCountsForValidate())
}

func (s *IntegrationSuite) TestValidateContract_IgnoresStoredPairVerdicts() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	s.publishAndDeployForValidate("web", "v1", validateWebIntegerItemsYAML)

	status, body := s.post("/api/can-i-deploy", `{"participant":"web","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, status, body)
	s.Require().Contains(body, `"deployable":false`)
	s.Require().Equal(1, s.countRows("compatibility_verdicts"))

	response, _ := s.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	s.True(response.Deployable)
	s.Empty(response.Results["catalog"].Endpoints)
	s.Equal(1, s.countRows("compatibility_verdicts"))
}

func (s *IntegrationSuite) TestValidateContract_RemovedResourceStillConsumedBlocks() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	s.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	s.False(response.Deployable)
	s.Require().Len(response.Results, 1)
	web := response.Results["web"]
	s.False(web.Deployable)
	s.Require().NotNil(web.ParticipantVersion)
	s.Equal("v1", *web.ParticipantVersion)

	s.Require().Len(web.Endpoints, 1)
	breaks := web.Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal("provider_resource_removed_but_still_consumed", breaks[0].Reason)
}

func (s *IntegrationSuite) TestValidateContract_NothingRemovedWhenParticipantIsNotDeployed() {
	s.setupForValidate("catalog", "web")
	s.mustPostForValidate("/api/contracts",
		s.publishBody("catalog", "v1", contractFragment{"api.yaml", validateCatalogItemsAndStockYAML}))
	s.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	s.True(response.Deployable)
	s.Empty(response.Results)
}

func (s *IntegrationSuite) TestValidateContract_ConsumerOfNeverOfferedResourceIsIgnored() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogStockYAML)
	s.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	s.True(response.Deployable)
	s.NotContains(response.Results, "web")
}

func (s *IntegrationSuite) TestValidateContract_ResourceAlreadyRemovedInDeployedVersionDoesNotCount() {
	s.setupForValidate("catalog", "web")
	s.mustPostForValidate("/api/contracts",
		s.publishBody("catalog", "v1", contractFragment{"api.yaml", validateCatalogItemsAndStockYAML}))
	s.publishAndDeployForValidate("catalog", "v2", validateCatalogStockYAML)
	s.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	s.True(response.Deployable)
	s.Empty(response.Results)
}

func (s *IntegrationSuite) TestValidateContract_RemovalIsMeasuredFromTheDeployedVersion() {
	s.setupForValidate("catalog", "web")
	s.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	s.mustPostForValidate("/api/contracts",
		s.publishBody("catalog", "v2", contractFragment{"api.yaml", validateCatalogStockYAML}))
	s.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := s.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	s.False(response.Deployable)
	breaks := response.Results["web"].Endpoints["/items"]["get"]["200"]
	s.Require().Len(breaks, 1)
	s.Equal("provider_resource_removed_but_still_consumed", breaks[0].Reason)
}

func (s *IntegrationSuite) TestValidateContract_DeployedVersionWithoutContractIsAnInternalError() {
	s.setupForValidate("catalog")
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO deployments (participant_id, version, environment_id, rollback)
		 SELECT p.id, 'v9', e.id, false FROM participants p, environments e
		  WHERE p.name = 'catalog' AND e.name = 'production'`)
	s.Require().NoError(err)

	status, body := s.post("/api/contracts/validate",
		s.validateBody("catalog", "production", validateFile{"api.yaml", validateCatalogStockYAML}))

	s.Equal(http.StatusInternalServerError, status)
	s.JSONEq(`{"message":"internal error"}`, body)
}
