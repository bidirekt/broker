package integration_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

type persistedVerdictBreak struct {
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Interaction string            `json:"interaction"`
	Reason      string            `json:"reason"`
	Details     map[string]string `json:"details"`
}

type persistedCheckResult struct {
	CounterpartName          string
	CounterpartParticipantID *int64
	CounterpartVersion       *string
	VerdictContractIDOne     *int64
	VerdictContractIDTwo     *int64
	Deployable               bool
}

const persistenceStandaloneContract = `
{
  "provides": { "rest": { "/widgets": { "get": { "responses": { "200": "Widget" } } } } },
  "schemas": { "Widget": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const persistenceGhostConsumerContract = `
{
  "consumes": { "ghost": { "rest": { "/ghosts": { "get": { "responses": { "200": "Ghost" } } } } } },
  "schemas": { "Ghost": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const persistenceWidgetsProviderContract = `
{
  "provides": { "rest": { "/widgets": { "get": { "responses": { "200": "Widget" } } } } },
  "schemas": { "Widget": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const persistenceWidgetsConsumerContract = `
{
  "consumes": { "widgets": { "rest": { "/widgets": { "get": { "responses": { "200": "Widget" } } } } } },
  "schemas": { "Widget": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const persistenceWidgetsBreakingConsumerContract = `
{
  "consumes": { "widgets": { "rest": { "/widgets": { "get": { "responses": { "200": "Widget" } } } } } },
  "schemas": {
    "Widget": {
      "type": "object",
      "properties": {
        "id":    { "type": "integer" },
        "label": { "type": "string" }
      }
    }
  }
}`

func (this *IntegrationSuite) mustPostForPersistence(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) participantIDForPersistence(name string) int64 {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name).Scan(&id))
	return id
}

func (this *IntegrationSuite) environmentIDForPersistence(name string) int64 {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM environments WHERE name = $1`, name).Scan(&id))
	return id
}

func (this *IntegrationSuite) contractIDForPersistence(participant, version string) int64 {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT cv.contract_id
		   FROM contract_versions cv
		   JOIN participants p ON p.id = cv.participant_id
		  WHERE p.name = $1 AND cv.version = $2`, participant, version).Scan(&id))
	return id
}

func (this *IntegrationSuite) singleCheckResultForPersistence() persistedCheckResult {
	var result persistedCheckResult
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT counterpart_name, counterpart_participant_id, counterpart_version,
		        verdict_contract_id_one, verdict_contract_id_two, deployable
		   FROM compatibility_check_results`).
		Scan(
			&result.CounterpartName,
			&result.CounterpartParticipantID,
			&result.CounterpartVersion,
			&result.VerdictContractIDOne,
			&result.VerdictContractIDTwo,
			&result.Deployable,
		))
	return result
}

func (this *IntegrationSuite) TestCompatibilityPersistence_BootstrapCheckRecordsNoCounterparts() {
	this.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceStandaloneContract}))

	status, body := this.post("/api/can-i-deploy",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.True(got.Deployable)
	this.Empty(got.Results)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(0, this.countRows("compatibility_check_results"))
	this.Equal(0, this.countRows("compatibility_verdicts"))

	var participantID, contractID, environmentID int64
	var version string
	var deployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT participant_id, contract_id, version, environment_id, deployable
		   FROM compatibility_checks`).
		Scan(&participantID, &contractID, &version, &environmentID, &deployable))

	this.Equal(this.participantIDForPersistence("widgets"), participantID)
	this.Equal(this.contractIDForPersistence("widgets", "v1"), contractID)
	this.Equal(this.environmentIDForPersistence("production"), environmentID)
	this.Equal("v1", version)
	this.True(deployable)
}

func (this *IntegrationSuite) TestCompatibilityPersistence_NotFoundCounterpartKeepsItsName() {
	this.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("front", "v1", contractFragment{"api.yaml", persistenceGhostConsumerContract}))

	status, body := this.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("provider_resource_not_found",
		got.Results["ghost"].Endpoints["/ghosts"]["get"]["200"][0].Reason)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_check_results"))
	this.Equal(0, this.countRows("compatibility_verdicts"))

	result := this.singleCheckResultForPersistence()
	this.Equal("ghost", result.CounterpartName)
	this.Nil(result.CounterpartParticipantID)
	this.Nil(result.CounterpartVersion)
	this.Nil(result.VerdictContractIDOne)
	this.Nil(result.VerdictContractIDTwo)
	this.False(result.Deployable)
}

func (this *IntegrationSuite) TestCompatibilityPersistence_NotDeployedCounterpartKeepsItsParticipant() {
	this.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	this.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsConsumerContract}))

	status, body := this.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("provider_resource_not_deployed_in_environment",
		got.Results["widgets"].Endpoints["/widgets"]["get"]["200"][0].Reason)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_check_results"))
	this.Equal(0, this.countRows("compatibility_verdicts"))

	result := this.singleCheckResultForPersistence()
	this.Equal("widgets", result.CounterpartName)
	this.Require().NotNil(result.CounterpartParticipantID)
	this.Equal(this.participantIDForPersistence("widgets"), *result.CounterpartParticipantID)
	this.Nil(result.CounterpartVersion)
	this.Nil(result.VerdictContractIDOne)
	this.Nil(result.VerdictContractIDTwo)
	this.False(result.Deployable)
}

func (this *IntegrationSuite) TestCompatibilityPersistence_CompatiblePairIsStoredWithEmptyBreaks() {
	this.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	this.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	this.mustPostForPersistence("/api/deployments",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsConsumerContract}))

	status, body := this.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.True(got.Deployable)

	this.Equal(1, this.countRows("compatibility_verdicts"))

	frontContractID := this.contractIDForPersistence("front", "v1")
	widgetsContractID := this.contractIDForPersistence("widgets", "v1")
	expectedOne, expectedTwo := model.OrderContractPair(frontContractID, widgetsContractID)

	var contractIDOne, contractIDTwo int64
	var breaks string
	var deployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id_one, contract_id_two, breaks::text, deployable
		   FROM compatibility_verdicts`).
		Scan(&contractIDOne, &contractIDTwo, &breaks, &deployable))

	this.Equal(expectedOne, contractIDOne)
	this.Equal(expectedTwo, contractIDTwo)
	this.Equal("[]", breaks)
	this.True(deployable)

	result := this.singleCheckResultForPersistence()
	this.Require().NotNil(result.VerdictContractIDOne)
	this.Require().NotNil(result.VerdictContractIDTwo)
	this.Equal(expectedOne, *result.VerdictContractIDOne)
	this.Equal(expectedTwo, *result.VerdictContractIDTwo)
	this.True(result.Deployable)
}

func (this *IntegrationSuite) TestCompatibilityPersistence_IncompatiblePairStoresEveryBreakKey() {
	this.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	this.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	this.mustPostForPersistence("/api/deployments",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsBreakingConsumerContract}))

	status, body := this.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)

	this.Equal(1, this.countRows("compatibility_verdicts"))

	var breaks []byte
	var deployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT breaks, deployable FROM compatibility_verdicts`).Scan(&breaks, &deployable))
	this.False(deployable)

	var stored []persistedVerdictBreak
	this.Require().NoError(json.Unmarshal(breaks, &stored))
	this.Require().Len(stored, 2)

	byReason := map[string]persistedVerdictBreak{}
	for _, item := range stored {
		byReason[item.Reason] = item
	}

	this.Equal(persistedVerdictBreak{
		Endpoint:    "/widgets",
		Method:      "get",
		Interaction: "200",
		Reason:      "property_type_mismatch",
		Details: map[string]string{
			"property":             "$.id",
			"consumerName":         "front",
			"providerName":         "widgets",
			"consumerPropertyType": "integer",
			"providerPropertyType": "string",
		},
	}, byReason["property_type_mismatch"])

	this.Equal(persistedVerdictBreak{
		Endpoint:    "/widgets",
		Method:      "get",
		Interaction: "200",
		Reason:      "property_missing_in_provider",
		Details: map[string]string{
			"property":     "$.label",
			"consumerName": "front",
			"providerName": "widgets",
			"propertyType": "string",
		},
	}, byReason["property_missing_in_provider"])
}

func (this *IntegrationSuite) TestCompatibilityPersistence_RepeatedCheckDoesNotGrowVerdicts() {
	this.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	this.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	this.mustPostForPersistence("/api/deployments",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsBreakingConsumerContract}))

	firstStatus, firstBody := this.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, firstStatus)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	secondStatus, secondBody := this.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	this.Require().Equal(http.StatusOK, secondStatus)
	this.Equal(firstBody, secondBody)

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(2, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))
}

func (this *IntegrationSuite) TestCompatibilityPersistence_RecordingTheSamePairTwiceKeepsOneVerdict() {
	this.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	this.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	this.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	this.mustPostForPersistence("/api/contracts",
		this.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsConsumerContract}))

	compatibilityRepository := repository.NewCompatibilityRepository(this.Pool)

	frontContractID := this.contractIDForPersistence("front", "v1")
	widgetsContractID := this.contractIDForPersistence("widgets", "v1")
	contractIDOne, contractIDTwo := model.OrderContractPair(frontContractID, widgetsContractID)

	recordCheck := func() {
		check := &model.CompatibilityCheck{
			ParticipantID: this.participantIDForPersistence("front"),
			ContractID:    frontContractID,
			Version:       "v1",
			EnvironmentID: this.environmentIDForPersistence("production"),
			Deployable:    true,
		}

		results := []model.CompatibilityCheckResult{{
			CounterpartName:          "widgets",
			CounterpartParticipantID: null.IntFrom(this.participantIDForPersistence("widgets")),
			CounterpartVersion:       null.StringFrom("v1"),
			VerdictContractIDOne:     null.IntFrom(contractIDOne),
			VerdictContractIDTwo:     null.IntFrom(contractIDTwo),
			Deployable:               true,
		}}

		newVerdicts := []model.CompatibilityVerdict{{
			ContractIDOne: contractIDTwo,
			ContractIDTwo: contractIDOne,
			Breaks:        []model.VerdictBreak{},
		}}

		compatibilityRepository.RecordCheck(context.Background(), check, results, newVerdicts)
	}

	recordCheck()
	recordCheck()

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(2, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	var storedOne, storedTwo int64
	var breaks string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id_one, contract_id_two, breaks::text FROM compatibility_verdicts`).
		Scan(&storedOne, &storedTwo, &breaks))
	this.Equal(contractIDOne, storedOne)
	this.Equal(contractIDTwo, storedTwo)
	this.Equal("[]", breaks)
}
