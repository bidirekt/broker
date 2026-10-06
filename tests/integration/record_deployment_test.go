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

func (this *IntegrationSuite) seedApiParticipantContractAndProductionEnv() {
	status, _ := this.post("/api/participants", apiParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ContractBody}))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", productionEnvBodyForDeploy)
	this.Require().Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) TestRecordDeployment_Success() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments", apiV1DeploymentBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment recorded"}`, body)

	this.Equal(1, this.countRows("deployments"))

	var (
		participantID int64
		version       string
		environmentID int64
	)

	err := this.Pool.QueryRow(context.Background(),
		`SELECT participant_id, version, environment_id FROM deployments LIMIT 1`,
	).Scan(&participantID, &version, &environmentID)
	this.Require().NoError(err)

	this.Equal("v1", version)
	this.Equal(this.lookupParticipantID("api"), participantID)
	this.Equal(this.lookupEnvironmentID("production"), environmentID)
}

func (this *IntegrationSuite) TestRecordDeployment_TwiceSameTupleIsIdempotent() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment recorded"}`, body)

	status, body = this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment recorded"}`, body)

	this.Equal(1, this.countRows("deployments"))

	var (
		participantID int64
		version       string
		environmentID int64
	)
	err := this.Pool.QueryRow(context.Background(),
		`SELECT participant_id, version, environment_id FROM deployments`,
	).Scan(&participantID, &version, &environmentID)
	this.Require().NoError(err)

	this.Equal("v1", version)
	this.Equal(this.lookupParticipantID("api"), participantID)
	this.Equal(this.lookupEnvironmentID("production"), environmentID)
}

func (this *IntegrationSuite) TestRecordDeployment_RollbackWritesNewRow() {
	status, _ := this.post("/api/participants", apiParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ContractBody}))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", apiV2ContractBody}))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", productionEnvBodyForDeploy)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/deployments", apiV2DeploymentBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)

	this.Equal(3, this.countRows("deployments"))

	rows, err := this.Pool.Query(context.Background(),
		`SELECT version, rollback FROM deployments ORDER BY deployed_at ASC`,
	)
	this.Require().NoError(err)
	defer rows.Close()

	type row struct {
		version  string
		rollback bool
	}
	var got []row
	for rows.Next() {
		var r row
		this.Require().NoError(rows.Scan(&r.version, &r.rollback))
		got = append(got, r)
	}
	this.Require().Len(got, 3)
	this.Equal(row{version: "v1", rollback: false}, got[0])
	this.Equal(row{version: "v2", rollback: false}, got[1])
	this.Equal(row{version: "v1", rollback: true}, got[2])
}

func (this *IntegrationSuite) TestRecordDeployment_MalformedJSONReturns400() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments", `{`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"deployment invalid input"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_MissingVersionReturns400() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments", `{"participant":"api","environment":"production"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"deployment invalid input"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_MissingEnvironmentReturns400() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments", `{"participant":"api","version":"v1"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"deployment invalid input"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_EmptyVersionReturns400() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments", `{"participant":"api","version":"","environment":"production"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"deployment invalid input"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_UnknownParticipantReturns404() {
	status, body := this.post("/api/deployments", `{"participant":"unknown","version":"v1","environment":"production"}`)
	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"participant not found"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_UnpublishedVersionReturns404() {
	status, _ := this.post("/api/participants", apiParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", productionEnvBodyForDeploy)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/deployments", apiV1DeploymentBody)
	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"version not found"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_UnknownEnvironmentReturns404() {
	status, _ := this.post("/api/participants", apiParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ContractBody}))
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/deployments", apiV1DeploymentBody)
	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"environment not found"}`, body)

	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_ExtraFieldsIgnored() {
	this.seedApiParticipantContractAndProductionEnv()

	status, body := this.post("/api/deployments",
		`{"participant":"api","version":"v1","environment":"production","deployer":"alice"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment recorded"}`, body)

	this.Equal(1, this.countRows("deployments"))

	var (
		participantID int64
		version       string
		environmentID int64
	)
	err := this.Pool.QueryRow(context.Background(),
		`SELECT participant_id, version, environment_id FROM deployments LIMIT 1`,
	).Scan(&participantID, &version, &environmentID)
	this.Require().NoError(err)
	this.Equal("v1", version)
	this.Equal(this.lookupParticipantID("api"), participantID)
	this.Equal(this.lookupEnvironmentID("production"), environmentID)
}

func (this *IntegrationSuite) lookupParticipantID(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name,
	).Scan(&id)
	this.Require().NoError(err)
	return id
}

func (this *IntegrationSuite) lookupEnvironmentID(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id FROM environments WHERE name = $1`, name,
	).Scan(&id)
	this.Require().NoError(err)
	return id
}
