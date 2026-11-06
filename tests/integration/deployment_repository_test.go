package integration_test

import (
	"context"
	"net/http"

	"github.com/bidirekt/broker/internal/repository"
)

const stagingEnvBody = `{"environment":"staging"}`

const deploymentRepositoryApiV1Contract = `
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

const deploymentRepositoryApiV2Contract = `provides:
  rest:
    /things:
      get:
        responses:
          200: Thing
schemas:
  Thing:
    type: object
    properties:
      id:
        type: string
      sku:
        type: string
`

func (this *IntegrationSuite) insertDeploymentAt(participantID int64, version string, environmentID int64, deployedAt string) {
	_, err := this.Pool.Exec(context.Background(),
		`WITH prior AS (
		     SELECT version FROM deployments
		     WHERE participant_id = $1 AND environment_id = $3
		 )
		 INSERT INTO deployments (participant_id, version, environment_id, rollback, deployed_at)
		 SELECT $1, $2, $3,
		        EXISTS (SELECT 1 FROM prior WHERE version = $2),
		        $4::timestamptz`,
		participantID, version, environmentID, deployedAt,
	)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(context.Background(),
		`INSERT INTO current_deployments (participant_id, environment_id, version, contract_id, deployed_at)
		 VALUES ($1, $3, $2,
		         (SELECT contract_id FROM contract_versions WHERE participant_id = $1 AND version = $2),
		         $4::timestamptz)
		 ON CONFLICT (participant_id, environment_id) DO UPDATE SET
		     version = EXCLUDED.version,
		     contract_id = EXCLUDED.contract_id,
		     deployed_at = EXCLUDED.deployed_at`,
		participantID, version, environmentID, deployedAt,
	)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) seedDeploymentRepositoryApiV1AndProduction() {
	status, _ := this.post("/api/participants", `{"participant":"api"}`)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", deploymentRepositoryApiV1Contract}))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) publishDeploymentRepositoryApiV2() {
	status, _ := this.post("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", deploymentRepositoryApiV2Contract}))
	this.Require().Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) contractIDOfVersion(participantID int64, version string) int64 {
	var contractID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_id FROM contract_versions WHERE participant_id = $1 AND version = $2`,
		participantID, version,
	).Scan(&contractID))

	return contractID
}

func (this *IntegrationSuite) participantIDForDeploymentRepository(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name,
	).Scan(&id)
	this.Require().NoError(err)
	return id
}

func (this *IntegrationSuite) environmentIDForDeploymentRepository(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id FROM environments WHERE name = $1`, name,
	).Scan(&id)
	this.Require().NoError(err)
	return id
}

func (this *IntegrationSuite) TestCurrentDeploymentInEnv_RollbackPicksLatestRowEvenIfOlderVersion() {
	this.seedDeploymentRepositoryApiV1AndProduction()
	this.publishDeploymentRepositoryApiV2()

	participantID := this.participantIDForDeploymentRepository("api")
	productionID := this.environmentIDForDeploymentRepository("production")

	this.insertDeploymentAt(participantID, "v1", productionID, "2026-05-01T00:00:00Z")
	this.insertDeploymentAt(participantID, "v2", productionID, "2026-05-10T00:00:00Z")
	this.insertDeploymentAt(participantID, "v1", productionID, "2026-05-15T00:00:00Z")

	repo := repository.NewDeploymentRepository(this.Pool)
	version, contractID, ok := repo.CurrentDeploymentInEnv(context.Background(), participantID, productionID)
	this.True(ok)
	this.Equal("v1", version)
	this.Equal(this.contractIDOfVersion(participantID, "v1"), contractID)
}

func (this *IntegrationSuite) TestCurrentDeploymentInEnv_NoRowsReturnsNotFound() {
	this.seedDeploymentRepositoryApiV1AndProduction()

	participantID := this.participantIDForDeploymentRepository("api")
	productionID := this.environmentIDForDeploymentRepository("production")

	repo := repository.NewDeploymentRepository(this.Pool)
	version, contractID, ok := repo.CurrentDeploymentInEnv(context.Background(), participantID, productionID)
	this.False(ok)
	this.Equal("", version)
	this.Zero(contractID)
}

func (this *IntegrationSuite) TestCurrentDeploymentInEnv_ScopedPerEnvironment() {
	this.seedDeploymentRepositoryApiV1AndProduction()

	this.publishDeploymentRepositoryApiV2()

	status, _ := this.post("/api/environments", stagingEnvBody)
	this.Require().Equal(http.StatusOK, status)

	participantID := this.participantIDForDeploymentRepository("api")
	productionID := this.environmentIDForDeploymentRepository("production")
	stagingID := this.environmentIDForDeploymentRepository("staging")

	this.insertDeploymentAt(participantID, "v1", productionID, "2026-05-01T00:00:00Z")
	this.insertDeploymentAt(participantID, "v2", stagingID, "2026-05-02T00:00:00Z")

	repo := repository.NewDeploymentRepository(this.Pool)

	prodVersion, prodContractID, prodOk := repo.CurrentDeploymentInEnv(context.Background(), participantID, productionID)
	this.True(prodOk)
	this.Equal("v1", prodVersion)
	this.Equal(this.contractIDOfVersion(participantID, "v1"), prodContractID)

	stagingVersion, stagingContractID, stagingOk := repo.CurrentDeploymentInEnv(context.Background(), participantID, stagingID)
	this.True(stagingOk)
	this.Equal("v2", stagingVersion)
	this.Equal(this.contractIDOfVersion(participantID, "v2"), stagingContractID)
}
