package integration_test

import (
	"context"
	"net/http"
)

const (
	apiParticipantBody         = `{"participant":"api"}`
	apiV1DeploymentBody        = `{"participant":"api","version":"v1","environment":"production"}`
	apiV2DeploymentBody        = `{"participant":"api","version":"v2","environment":"production"}`
	productionEnvBodyForDeploy = `{"environment":"production"}`
)

const apiV1ContractBody = `
{
  "provides": {
    "rest": {
      "/things": {
        "get": {
          "responses": {
            "200": "Thing"
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

const apiV2ContractBody = `
{
  "provides": {
    "rest": {
      "/things": {
        "get": {
          "responses": {
            "200": "Thing"
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

func (s *IntegrationSuite) seedApiParticipantContractAndProductionEnv() {
	status, _ := s.post("/api/participants", apiParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ContractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/environments", productionEnvBodyForDeploy)
	s.Require().Equal(http.StatusOK, status)
}

func (s *IntegrationSuite) TestRecordDeployment_Success() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments", apiV1DeploymentBody)
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"deployment recorded"}`, body)

	s.Equal(1, s.countRows("deployments"))

	var (
		participantID int64
		version       string
		environmentID int64
	)

	err := s.Pool.QueryRow(context.Background(),
		`SELECT participant_id, version, environment_id FROM deployments LIMIT 1`,
	).Scan(&participantID, &version, &environmentID)
	s.Require().NoError(err)

	s.Equal("v1", version)
	s.Equal(s.lookupParticipantID("api"), participantID)
	s.Equal(s.lookupEnvironmentID("production"), environmentID)
}

func (s *IntegrationSuite) TestRecordDeployment_TwiceSameTupleIsIdempotent() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments", apiV1DeploymentBody)
	s.Require().Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"deployment recorded"}`, body)

	status, body = s.post("/api/deployments", apiV1DeploymentBody)
	s.Require().Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"deployment recorded"}`, body)

	s.Equal(1, s.countRows("deployments"))

	var (
		participantID int64
		version       string
		environmentID int64
	)
	err := s.Pool.QueryRow(context.Background(),
		`SELECT participant_id, version, environment_id FROM deployments`,
	).Scan(&participantID, &version, &environmentID)
	s.Require().NoError(err)

	s.Equal("v1", version)
	s.Equal(s.lookupParticipantID("api"), participantID)
	s.Equal(s.lookupEnvironmentID("production"), environmentID)
}

func (s *IntegrationSuite) TestRecordDeployment_RollbackWritesNewRow() {
	status, _ := s.post("/api/participants", apiParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ContractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("api", "v2", contractFragment{"api.yaml", apiV2ContractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/environments", productionEnvBodyForDeploy)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/deployments", apiV1DeploymentBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/deployments", apiV2DeploymentBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/deployments", apiV1DeploymentBody)
	s.Require().Equal(http.StatusOK, status)

	s.Equal(3, s.countRows("deployments"))

	rows, err := s.Pool.Query(context.Background(),
		`SELECT version, rollback FROM deployments ORDER BY deployed_at ASC`,
	)
	s.Require().NoError(err)
	defer rows.Close()

	type row struct {
		version  string
		rollback bool
	}
	var got []row
	for rows.Next() {
		var r row
		s.Require().NoError(rows.Scan(&r.version, &r.rollback))
		got = append(got, r)
	}
	s.Require().Len(got, 3)
	s.Equal(row{version: "v1", rollback: false}, got[0])
	s.Equal(row{version: "v2", rollback: false}, got[1])
	s.Equal(row{version: "v1", rollback: true}, got[2])
}

func (s *IntegrationSuite) TestRecordDeployment_MalformedJSONReturns400() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments", `{`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"deployment invalid input"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_MissingVersionReturns400() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments", `{"participant":"api","environment":"production"}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"deployment invalid input"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_MissingEnvironmentReturns400() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments", `{"participant":"api","version":"v1"}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"deployment invalid input"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_EmptyVersionReturns400() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments", `{"participant":"api","version":"","environment":"production"}`)
	s.Equal(http.StatusBadRequest, status)
	s.JSONEq(`{"message":"deployment invalid input"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_UnknownParticipantReturns404() {
	status, body := s.post("/api/deployments", `{"participant":"unknown","version":"v1","environment":"production"}`)
	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"participant not found"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_UnpublishedVersionReturns404() {
	status, _ := s.post("/api/participants", apiParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/environments", productionEnvBodyForDeploy)
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/deployments", apiV1DeploymentBody)
	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"version not found"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_UnknownEnvironmentReturns404() {
	status, _ := s.post("/api/participants", apiParticipantBody)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ContractBody}))
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/deployments", apiV1DeploymentBody)
	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"environment not found"}`, body)

	s.Equal(0, s.countRows("deployments"))
}

func (s *IntegrationSuite) TestRecordDeployment_ExtraFieldsIgnored() {
	s.seedApiParticipantContractAndProductionEnv()

	status, body := s.post("/api/deployments",
		`{"participant":"api","version":"v1","environment":"production","deployer":"alice"}`)
	s.Equal(http.StatusOK, status)
	s.JSONEq(`{"message":"deployment recorded"}`, body)

	s.Equal(1, s.countRows("deployments"))

	var (
		participantID int64
		version       string
		environmentID int64
	)
	err := s.Pool.QueryRow(context.Background(),
		`SELECT participant_id, version, environment_id FROM deployments LIMIT 1`,
	).Scan(&participantID, &version, &environmentID)
	s.Require().NoError(err)
	s.Equal("v1", version)
	s.Equal(s.lookupParticipantID("api"), participantID)
	s.Equal(s.lookupEnvironmentID("production"), environmentID)
}

func (s *IntegrationSuite) lookupParticipantID(name string) int64 {
	var id int64
	err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name,
	).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *IntegrationSuite) lookupEnvironmentID(name string) int64 {
	var id int64
	err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM environments WHERE name = $1`, name,
	).Scan(&id)
	s.Require().NoError(err)
	return id
}
