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

func (s *IntegrationSuite) mustPostForPersistence(path, body string) {
	status, response := s.post(path, body)
	s.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (s *IntegrationSuite) participantIDForPersistence(name string) int64 {
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name).Scan(&id))
	return id
}

func (s *IntegrationSuite) environmentIDForPersistence(name string) int64 {
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM environments WHERE name = $1`, name).Scan(&id))
	return id
}

func (s *IntegrationSuite) contractIDForPersistence(participant, version string) int64 {
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT cv.contract_id
		   FROM contract_versions cv
		   JOIN participants p ON p.id = cv.participant_id
		  WHERE p.name = $1 AND cv.version = $2`, participant, version).Scan(&id))
	return id
}

func (s *IntegrationSuite) singleCheckResultForPersistence() persistedCheckResult {
	var result persistedCheckResult
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
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

func (s *IntegrationSuite) TestCompatibilityPersistence_BootstrapCheckRecordsNoCounterparts() {
	s.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceStandaloneContract}))

	status, body := s.post("/api/can-i-deploy",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.True(got.Deployable)
	s.Empty(got.Results)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(0, s.countRows("compatibility_check_results"))
	s.Equal(0, s.countRows("compatibility_verdicts"))

	var participantID, contractID, environmentID int64
	var version string
	var deployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT participant_id, contract_id, version, environment_id, deployable
		   FROM compatibility_checks`).
		Scan(&participantID, &contractID, &version, &environmentID, &deployable))

	s.Equal(s.participantIDForPersistence("widgets"), participantID)
	s.Equal(s.contractIDForPersistence("widgets", "v1"), contractID)
	s.Equal(s.environmentIDForPersistence("production"), environmentID)
	s.Equal("v1", version)
	s.True(deployable)
}

func (s *IntegrationSuite) TestCompatibilityPersistence_NotFoundCounterpartKeepsItsName() {
	s.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("front", "v1", contractFragment{"api.yaml", persistenceGhostConsumerContract}))

	status, body := s.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("provider_resource_not_found",
		got.Results["ghost"].Endpoints["/ghosts"]["get"]["200"][0].Reason)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(1, s.countRows("compatibility_check_results"))
	s.Equal(0, s.countRows("compatibility_verdicts"))

	result := s.singleCheckResultForPersistence()
	s.Equal("ghost", result.CounterpartName)
	s.Nil(result.CounterpartParticipantID)
	s.Nil(result.CounterpartVersion)
	s.Nil(result.VerdictContractIDOne)
	s.Nil(result.VerdictContractIDTwo)
	s.False(result.Deployable)
}

func (s *IntegrationSuite) TestCompatibilityPersistence_NotDeployedCounterpartKeepsItsParticipant() {
	s.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	s.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsConsumerContract}))

	status, body := s.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("provider_resource_not_deployed_in_environment",
		got.Results["widgets"].Endpoints["/widgets"]["get"]["200"][0].Reason)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(1, s.countRows("compatibility_check_results"))
	s.Equal(0, s.countRows("compatibility_verdicts"))

	result := s.singleCheckResultForPersistence()
	s.Equal("widgets", result.CounterpartName)
	s.Require().NotNil(result.CounterpartParticipantID)
	s.Equal(s.participantIDForPersistence("widgets"), *result.CounterpartParticipantID)
	s.Nil(result.CounterpartVersion)
	s.Nil(result.VerdictContractIDOne)
	s.Nil(result.VerdictContractIDTwo)
	s.False(result.Deployable)
}

func (s *IntegrationSuite) TestCompatibilityPersistence_CompatiblePairIsStoredWithEmptyBreaks() {
	s.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	s.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	s.mustPostForPersistence("/api/deployments",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsConsumerContract}))

	status, body := s.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.True(got.Deployable)

	s.Equal(1, s.countRows("compatibility_verdicts"))

	frontContractID := s.contractIDForPersistence("front", "v1")
	widgetsContractID := s.contractIDForPersistence("widgets", "v1")
	expectedOne, expectedTwo := model.OrderContractPair(frontContractID, widgetsContractID)

	var contractIDOne, contractIDTwo int64
	var breaks string
	var deployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT contract_id_one, contract_id_two, breaks::text, deployable
		   FROM compatibility_verdicts`).
		Scan(&contractIDOne, &contractIDTwo, &breaks, &deployable))

	s.Equal(expectedOne, contractIDOne)
	s.Equal(expectedTwo, contractIDTwo)
	s.Equal("[]", breaks)
	s.True(deployable)

	result := s.singleCheckResultForPersistence()
	s.Require().NotNil(result.VerdictContractIDOne)
	s.Require().NotNil(result.VerdictContractIDTwo)
	s.Equal(expectedOne, *result.VerdictContractIDOne)
	s.Equal(expectedTwo, *result.VerdictContractIDTwo)
	s.True(result.Deployable)
}

func (s *IntegrationSuite) TestCompatibilityPersistence_IncompatiblePairStoresEveryBreakKey() {
	s.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	s.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	s.mustPostForPersistence("/api/deployments",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsBreakingConsumerContract}))

	status, body := s.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)

	s.Equal(1, s.countRows("compatibility_verdicts"))

	var breaks []byte
	var deployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT breaks, deployable FROM compatibility_verdicts`).Scan(&breaks, &deployable))
	s.False(deployable)

	var stored []persistedVerdictBreak
	s.Require().NoError(json.Unmarshal(breaks, &stored))
	s.Require().Len(stored, 2)

	byReason := map[string]persistedVerdictBreak{}
	for _, item := range stored {
		byReason[item.Reason] = item
	}

	s.Equal(persistedVerdictBreak{
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

	s.Equal(persistedVerdictBreak{
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

func (s *IntegrationSuite) TestCompatibilityPersistence_RepeatedCheckDoesNotGrowVerdicts() {
	s.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	s.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	s.mustPostForPersistence("/api/deployments",
		`{"participant":"widgets","version":"v1","environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsBreakingConsumerContract}))

	firstStatus, firstBody := s.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, firstStatus)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(1, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	secondStatus, secondBody := s.post("/api/can-i-deploy",
		`{"participant":"front","version":"v1","environment":"production"}`)
	s.Require().Equal(http.StatusOK, secondStatus)
	s.Equal(firstBody, secondBody)

	s.Equal(2, s.countRows("compatibility_checks"))
	s.Equal(2, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))
}

func (s *IntegrationSuite) TestCompatibilityPersistence_RecordingTheSamePairTwiceKeepsOneVerdict() {
	s.mustPostForPersistence("/api/participants", `{"participant":"widgets"}`)
	s.mustPostForPersistence("/api/participants", `{"participant":"front"}`)
	s.mustPostForPersistence("/api/environments", `{"environment":"production"}`)
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("widgets", "v1", contractFragment{"api.yaml", persistenceWidgetsProviderContract}))
	s.mustPostForPersistence("/api/contracts",
		s.publishBody("front", "v1", contractFragment{"api.yaml", persistenceWidgetsConsumerContract}))

	compatibilityRepository := repository.NewCompatibilityRepository(s.Pool)

	frontContractID := s.contractIDForPersistence("front", "v1")
	widgetsContractID := s.contractIDForPersistence("widgets", "v1")
	contractIDOne, contractIDTwo := model.OrderContractPair(frontContractID, widgetsContractID)

	recordCheck := func() {
		check := &model.CompatibilityCheck{
			ParticipantID: s.participantIDForPersistence("front"),
			ContractID:    frontContractID,
			Version:       "v1",
			EnvironmentID: s.environmentIDForPersistence("production"),
			Deployable:    true,
		}

		results := []model.CompatibilityCheckResult{{
			CounterpartName:          "widgets",
			CounterpartParticipantID: null.IntFrom(s.participantIDForPersistence("widgets")),
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

	s.Equal(2, s.countRows("compatibility_checks"))
	s.Equal(2, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	var storedOne, storedTwo int64
	var breaks string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT contract_id_one, contract_id_two, breaks::text FROM compatibility_verdicts`).
		Scan(&storedOne, &storedTwo, &breaks))
	s.Equal(contractIDOne, storedOne)
	s.Equal(contractIDTwo, storedTwo)
	s.Equal("[]", breaks)
}
