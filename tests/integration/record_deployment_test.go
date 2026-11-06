package integration_test

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	apiParticipantBody         = `{"participant":"api"}`
	apiV1DeploymentBody        = `{"participant":"api","version":"v1","environment":"production"}`
	apiV2DeploymentBody        = `{"participant":"api","version":"v2","environment":"production"}`
	apiV1StagingDeploymentBody = `{"participant":"api","version":"v1","environment":"staging"}`
	productionEnvBodyForDeploy = `{"environment":"production"}`
	stagingEnvBodyForDeploy    = `{"environment":"staging"}`
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

func recordDeploymentInBackground(write func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		write()
	}()

	return panicked
}

func (this *IntegrationSuite) recordDeploymentConnect() *pgx.Conn {
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, this.Pool.Config().ConnConfig)
	this.Require().NoError(err)
	this.T().Cleanup(func() { _ = conn.Close(ctx) })

	return conn
}

func (this *IntegrationSuite) recordDeploymentAwaitLockWaiters(count int) {
	lockWaiters := this.recordDeploymentConnect()

	this.Require().Eventually(func() bool {
		var waiting int
		err := lockWaiters.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) TestRecordDeployment_ConcurrentSameTuple_RecordsOneRow() {
	this.seedApiParticipantContractAndProductionEnv()

	ctx := context.Background()
	participantOnHold, err := this.recordDeploymentConnect().Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = participantOnHold.Rollback(ctx) }()
	_, err = participantOnHold.Exec(ctx, `SELECT 1 FROM participants WHERE id = $1 FOR UPDATE`, this.lookupParticipantID("api"))
	this.Require().NoError(err)

	const deploys = 3
	statuses := make([]int, deploys)
	bodies := make([]string, deploys)
	finished := make([]<-chan any, deploys)
	for i := range deploys {
		finished[i] = recordDeploymentInBackground(func() {
			statuses[i], bodies[i] = this.post("/api/deployments", apiV1DeploymentBody)
		})
	}
	this.recordDeploymentAwaitLockWaiters(deploys)

	this.Require().NoError(participantOnHold.Rollback(ctx))
	for i := range deploys {
		this.Nil(<-finished[i])
		this.Equal(http.StatusOK, statuses[i])
		this.JSONEq(`{"message":"deployment recorded"}`, bodies[i])
	}

	this.Equal(1, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestRecordDeployment_WaitingOnTheParticipantLock_IsTimestampedAfterTheWait() {
	this.seedApiParticipantContractAndProductionEnv()

	ctx := context.Background()
	participantOnHold, err := this.recordDeploymentConnect().Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = participantOnHold.Rollback(ctx) }()
	_, err = participantOnHold.Exec(ctx, `SELECT 1 FROM participants WHERE id = $1 FOR UPDATE`, this.lookupParticipantID("api"))
	this.Require().NoError(err)

	var status int
	finished := recordDeploymentInBackground(func() {
		status, _ = this.post("/api/deployments", apiV1DeploymentBody)
	})
	this.recordDeploymentAwaitLockWaiters(1)

	var duringTheWait time.Time
	this.Require().NoError(participantOnHold.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&duringTheWait))
	this.Require().NoError(participantOnHold.Rollback(ctx))
	this.Require().Nil(<-finished)
	this.Require().Equal(http.StatusOK, status)

	var deployedAt time.Time
	this.Require().NoError(this.Pool.QueryRow(ctx, `SELECT deployed_at FROM deployments`).Scan(&deployedAt))
	this.Greater(deployedAt, duringTheWait)
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

	status, body := this.post("/api/deployments", apiV2DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment recorded"}`, body)

	status, body = this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment rolled back"}`, body)

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

type recordDeploymentCurrentRow struct {
	version    string
	contractID int64
	deployedAt time.Time
}

func (this *IntegrationSuite) recordDeploymentCurrentRows() []recordDeploymentCurrentRow {
	rows, err := this.Pool.Query(context.Background(),
		`SELECT version, contract_id, deployed_at FROM current_deployments`,
	)
	this.Require().NoError(err)

	current, err := pgx.AppendRows(make([]recordDeploymentCurrentRow, 0), rows, func(row pgx.CollectableRow) (recordDeploymentCurrentRow, error) {
		var current recordDeploymentCurrentRow
		err := row.Scan(&current.version, &current.contractID, &current.deployedAt)

		return current, err
	})
	this.Require().NoError(err)

	return current
}

func (this *IntegrationSuite) recordDeploymentLatestRow() (version string, deployedAt time.Time) {
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT version, deployed_at FROM deployments ORDER BY deployed_at DESC LIMIT 1`,
	).Scan(&version, &deployedAt))

	return version, deployedAt
}

func (this *IntegrationSuite) recordDeploymentContractIDOf(version string) int64 {
	var contractID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id FROM contract_versions WHERE version = $1`, version,
	).Scan(&contractID))

	return contractID
}

func (this *IntegrationSuite) TestRecordDeployment_CurrentDeploymentFollowsNewSameAndRolledBackVersions() {
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
	_, v1DeployedAt := this.recordDeploymentLatestRow()
	this.Equal([]recordDeploymentCurrentRow{{"v1", this.recordDeploymentContractIDOf("v1"), v1DeployedAt}}, this.recordDeploymentCurrentRows(), "first deployment")

	status, _ = this.post("/api/deployments", apiV2DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	_, v2DeployedAt := this.recordDeploymentLatestRow()
	this.Equal([]recordDeploymentCurrentRow{{"v2", this.recordDeploymentContractIDOf("v2"), v2DeployedAt}}, this.recordDeploymentCurrentRows(), "new version")

	status, _ = this.post("/api/deployments", apiV2DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	this.Equal(2, this.countRows("deployments"), "same version")
	this.Equal([]recordDeploymentCurrentRow{{"v2", this.recordDeploymentContractIDOf("v2"), v2DeployedAt}}, this.recordDeploymentCurrentRows(), "same version")

	status, body := this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment rolled back"}`, body)
	_, rollbackDeployedAt := this.recordDeploymentLatestRow()
	this.Equal([]recordDeploymentCurrentRow{{"v1", this.recordDeploymentContractIDOf("v1"), rollbackDeployedAt}}, this.recordDeploymentCurrentRows(), "rollback")
}

func (this *IntegrationSuite) TestRecordDeployment_RollbackScopedPerEnvironment() {
	this.seedApiParticipantContractAndProductionEnv()

	status, _ := this.post("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", apiV2ContractBody}))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", stagingEnvBodyForDeploy)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/deployments", apiV1DeploymentBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/deployments", apiV2DeploymentBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/deployments", apiV1StagingDeploymentBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"deployment recorded"}`, body)
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

func (this *IntegrationSuite) TestRecordDeployment_PaddedParticipantVersionOrEnvironmentReturns400() {
	this.seedApiParticipantContractAndProductionEnv()

	cases := map[string]string{
		"trailing space in participant":   `{"participant":"api ","version":"v1","environment":"production"}`,
		"leading space in version":        `{"participant":"api","version":" v1","environment":"production"}`,
		"trailing newline in environment": `{"participant":"api","version":"v1","environment":"production\n"}`,
	}

	for name, body := range cases {
		status, response := this.post("/api/deployments", body)
		this.Equal(http.StatusBadRequest, status, name)
		this.JSONEq(`{"message":"deployment invalid input"}`, response, name)
	}

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
