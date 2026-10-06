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

func (this *IntegrationSuite) mustPostForValidate(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) validateBody(participant, environment string, files ...validateFile) string {
	body, err := json.Marshal(struct {
		Participant string         `json:"participant"`
		Environment string         `json:"environment"`
		Contracts   []validateFile `json:"contracts"`
	}{
		Participant: participant,
		Environment: environment,
		Contracts:   files,
	})
	this.Require().NoError(err)

	return string(body)
}

func (this *IntegrationSuite) validateOK(participant string, files ...validateFile) (validateResponseJSON, string) {
	status, body := this.post("/api/contracts/validate", this.validateBody(participant, "production", files...))
	this.Require().Equalf(http.StatusOK, status, "validate %s: %s", participant, body)

	var response validateResponseJSON
	this.Require().NoError(json.Unmarshal([]byte(body), &response))
	this.Equal("contract validated successfully", response.Message)
	this.Equal(participant, response.Participant)
	this.Equal("production", response.Environment)

	return response, body
}

func (this *IntegrationSuite) publishAndDeployForValidate(participant, version, content string) {
	this.mustPostForValidate("/api/contracts", this.publishBody(participant, version, contractFragment{"api.yaml", content}))
	this.mustPostForValidate("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) setupForValidate(participants ...string) {
	this.mustPostForValidate("/api/environments", `{"environment":"production"}`)
	for _, participant := range participants {
		this.mustPostForValidate("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) rowCountsForValidate() map[string]int {
	counts := make(map[string]int, len(validateTables))
	for _, table := range validateTables {
		counts[table] = this.countRows(table)
	}

	return counts
}

func (this *IntegrationSuite) TestValidateContract_InvalidInput() {
	this.setupForValidate("catalog")

	file := validateFile{"api.yaml", validateCatalogStockYAML}
	cases := map[string]string{
		"unparseable body":    `{"participant":`,
		"empty participant":   this.validateBody(" ", "production", file),
		"empty environment":   this.validateBody("catalog", "", file),
		"empty contracts":     this.validateBody("catalog", "production"),
		"blank source":        this.validateBody("catalog", "production", validateFile{"  ", validateCatalogStockYAML}),
		"blank unknown names": this.validateBody("ghost", "nowhere", validateFile{"", ""}, validateFile{"notes.txt", "x"}),
	}

	for name, body := range cases {
		status, response := this.post("/api/contracts/validate", body)
		this.Equal(http.StatusBadRequest, status, name)
		this.JSONEq(`{"message":"contract invalid input"}`, response, name)
	}
}

func (this *IntegrationSuite) TestValidateContract_DecodeErrorComesBeforeParticipantNotFound() {
	status, body := this.post("/api/contracts/validate",
		this.validateBody("ghost", "nowhere", validateFile{"notes.txt", validateCatalogStockYAML}))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"unsupported contract file: notes.txt (expected .yaml or .yml)"}`, body)

	status, body = this.post("/api/contracts/validate",
		this.validateBody("ghost", "nowhere", validateFile{"broken.yaml", "provides: {"}))
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, `"message":"malformed contract file: broken.yaml:`)
}

func (this *IntegrationSuite) TestValidateContract_ShapeViolationComesBeforeParticipantNotFound() {
	status, body := this.post("/api/contracts/validate",
		this.validateBody("ghost", "nowhere", validateFile{"api.yaml", validateUnknownKeyYAML}))

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"key.unknown","path":"provides;message","source":"api.yaml","details":{"key":"message"}}`+
		`]}`, body)
}

func (this *IntegrationSuite) TestValidateContract_ParticipantNotFoundComesBeforeEnvironmentNotFound() {
	status, body := this.post("/api/contracts/validate",
		this.validateBody("ghost", "nowhere", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))

	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"participant not found"}`, body)
}

func (this *IntegrationSuite) TestValidateContract_EnvironmentNotFoundComesBeforeRuleViolation() {
	this.setupForValidate("catalog")

	status, body := this.post("/api/contracts/validate",
		this.validateBody("catalog", "nowhere", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))

	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"environment not found"}`, body)
}

func (this *IntegrationSuite) TestValidateContract_RuleViolation() {
	this.setupForValidate("catalog")

	status, body := this.post("/api/contracts/validate",
		this.validateBody(" catalog ", " production ", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))
	this.Equal(http.StatusBadRequest, status)

	var response struct {
		Message    string `json:"message"`
		Violations []struct {
			Code   string `json:"code"`
			Source string `json:"source"`
		} `json:"violations"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))
	this.Equal("contract validation failed", response.Message)
	this.Require().Len(response.Violations, 1)
	this.Equal("schema.unresolved_name", response.Violations[0].Code)
	this.Equal("api.yaml", response.Violations[0].Source)
}

func (this *IntegrationSuite) TestValidateContract_Compatible() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)

	response, _ := this.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	this.True(response.Deployable)
	this.Require().Len(response.Results, 1)
	catalog := response.Results["catalog"]
	this.True(catalog.Deployable)
	this.Empty(catalog.Endpoints)
	this.Require().NotNil(catalog.ParticipantVersion)
	this.Equal("v1", *catalog.ParticipantVersion)
}

func (this *IntegrationSuite) TestValidateContract_IncompatibleWithDeployedProvider() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)

	response, _ := this.validateOK("web", validateFile{"api.yaml", validateWebIntegerItemsYAML})

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	catalog := response.Results["catalog"]
	this.False(catalog.Deployable)

	breaks := catalog.Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("property_type_mismatch", breaks[0].Reason)
}

func (this *IntegrationSuite) TestValidateContract_ConsumerOfNeverPublishedProvider() {
	this.setupForValidate("catalog", "web")

	response, _ := this.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	this.False(response.Deployable)
	breaks := response.Results["catalog"].Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("provider_resource_not_found", breaks[0].Reason)
}

func (this *IntegrationSuite) TestValidateContract_ConsumerOfProviderNotDeployedInEnvironment() {
	this.setupForValidate("catalog", "web")
	this.mustPostForValidate("/api/contracts",
		this.publishBody("catalog", "v1", contractFragment{"api.yaml", validateCatalogItemsAndStockYAML}))

	response, _ := this.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	this.False(response.Deployable)
	breaks := response.Results["catalog"].Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("provider_resource_not_deployed_in_environment", breaks[0].Reason)
}

func (this *IntegrationSuite) TestValidateContract_IgnoresPublishedButNotDeployedVersions() {
	this.setupForValidate("catalog", "web", "billing")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	this.mustPostForValidate("/api/contracts",
		this.publishBody("catalog", "v2", contractFragment{"api.yaml", validateCatalogIntegerItemsYAML}))
	this.mustPostForValidate("/api/contracts",
		this.publishBody("billing", "v1", contractFragment{"api.yaml", validateWebIntegerItemsYAML}))

	consumer, _ := this.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})
	this.True(consumer.Deployable)
	this.Require().NotNil(consumer.Results["catalog"].ParticipantVersion)
	this.Equal("v1", *consumer.Results["catalog"].ParticipantVersion)

	provider, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogItemsAndStockYAML})
	this.True(provider.Deployable)
	this.NotContains(provider.Results, "billing")
}

func (this *IntegrationSuite) TestValidateContract_WithoutConsumesInEnvironmentWithoutConsumers() {
	this.setupForValidate("catalog")

	response, body := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogItemsAndStockYAML})

	this.True(response.Deployable)
	this.Empty(response.Results)
	this.JSONEq(`{"message":"contract validated successfully","participant":"catalog","environment":"production","deployable":true,"results":{}}`, body)
}

func (this *IntegrationSuite) TestValidateContract_StoresNothing() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	this.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	before := this.rowCountsForValidate()

	compatible, _ := this.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})
	this.True(compatible.Deployable)
	this.Equal(before, this.rowCountsForValidate())

	incompatible, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogIntegerItemsYAML})
	this.False(incompatible.Deployable)
	this.Equal(before, this.rowCountsForValidate())

	status, _ := this.post("/api/contracts/validate",
		this.validateBody("catalog", "production", validateFile{"api.yaml", validateUnresolvedSchemaYAML}))
	this.Equal(http.StatusBadRequest, status)
	this.Equal(before, this.rowCountsForValidate())
}

func (this *IntegrationSuite) TestValidateContract_IgnoresStoredPairVerdicts() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	this.publishAndDeployForValidate("web", "v1", validateWebIntegerItemsYAML)

	status, body := this.post("/api/can-i-deploy", `{"participant":"web","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)
	this.Require().Contains(body, `"deployable":false`)
	this.Require().Equal(1, this.countRows("compatibility_verdicts"))

	response, _ := this.validateOK("web", validateFile{"api.yaml", validateWebItemsYAML})

	this.True(response.Deployable)
	this.Empty(response.Results["catalog"].Endpoints)
	this.Equal(1, this.countRows("compatibility_verdicts"))
}

func (this *IntegrationSuite) TestValidateContract_RemovedResourceStillConsumedBlocks() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	this.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	this.False(response.Deployable)
	this.Require().Len(response.Results, 1)
	web := response.Results["web"]
	this.False(web.Deployable)
	this.Require().NotNil(web.ParticipantVersion)
	this.Equal("v1", *web.ParticipantVersion)

	this.Require().Len(web.Endpoints, 1)
	breaks := web.Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("provider_resource_removed_but_still_consumed", breaks[0].Reason)
}

func (this *IntegrationSuite) TestValidateContract_NothingRemovedWhenParticipantIsNotDeployed() {
	this.setupForValidate("catalog", "web")
	this.mustPostForValidate("/api/contracts",
		this.publishBody("catalog", "v1", contractFragment{"api.yaml", validateCatalogItemsAndStockYAML}))
	this.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	this.True(response.Deployable)
	this.Empty(response.Results)
}

func (this *IntegrationSuite) TestValidateContract_ConsumerOfNeverOfferedResourceIsIgnored() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogStockYAML)
	this.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	this.True(response.Deployable)
	this.NotContains(response.Results, "web")
}

func (this *IntegrationSuite) TestValidateContract_ResourceAlreadyRemovedInDeployedVersionDoesNotCount() {
	this.setupForValidate("catalog", "web")
	this.mustPostForValidate("/api/contracts",
		this.publishBody("catalog", "v1", contractFragment{"api.yaml", validateCatalogItemsAndStockYAML}))
	this.publishAndDeployForValidate("catalog", "v2", validateCatalogStockYAML)
	this.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	this.True(response.Deployable)
	this.Empty(response.Results)
}

func (this *IntegrationSuite) TestValidateContract_RemovalIsMeasuredFromTheDeployedVersion() {
	this.setupForValidate("catalog", "web")
	this.publishAndDeployForValidate("catalog", "v1", validateCatalogItemsAndStockYAML)
	this.mustPostForValidate("/api/contracts",
		this.publishBody("catalog", "v2", contractFragment{"api.yaml", validateCatalogStockYAML}))
	this.publishAndDeployForValidate("web", "v1", validateWebItemsYAML)

	response, _ := this.validateOK("catalog", validateFile{"api.yaml", validateCatalogStockYAML})

	this.False(response.Deployable)
	breaks := response.Results["web"].Endpoints["/items"]["get"]["200"]
	this.Require().Len(breaks, 1)
	this.Equal("provider_resource_removed_but_still_consumed", breaks[0].Reason)
}

func (this *IntegrationSuite) TestValidateContract_DeployedVersionWithoutContractIsAnInternalError() {
	this.setupForValidate("catalog")
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO deployments (participant_id, version, environment_id, rollback)
		 SELECT p.id, 'v9', e.id, false FROM participants p, environments e
		  WHERE p.name = 'catalog' AND e.name = 'production'`)
	this.Require().NoError(err)

	status, body := this.post("/api/contracts/validate",
		this.validateBody("catalog", "production", validateFile{"api.yaml", validateCatalogStockYAML}))

	this.Equal(http.StatusInternalServerError, status)
	this.JSONEq(`{"message":"internal error"}`, body)
}
