package integration_test

import (
	"context"
	"net/http"

	"github.com/bidirekt/broker/internal/repository"
)

const stagingEnvBody = `{"environment":"staging"}`

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
}

func (this *IntegrationSuite) TestCurrentVersionInEnv_RollbackPicksLatestRowEvenIfOlderVersion() {
	this.seedApiParticipantContractAndProductionEnv()

	participantID := this.lookupParticipantID("api")
	productionID := this.lookupEnvironmentID("production")

	this.insertDeploymentAt(participantID, "v1", productionID, "2026-05-01T00:00:00Z")
	this.insertDeploymentAt(participantID, "v2", productionID, "2026-05-10T00:00:00Z")
	this.insertDeploymentAt(participantID, "v1", productionID, "2026-05-15T00:00:00Z")

	repo := repository.NewDeploymentRepository(this.Pool)
	version, ok := repo.CurrentVersionInEnv(context.Background(), participantID, productionID)
	this.True(ok)
	this.Equal("v1", version)
}

func (this *IntegrationSuite) TestCurrentVersionInEnv_NoRowsReturnsNotFound() {
	this.seedApiParticipantContractAndProductionEnv()

	participantID := this.lookupParticipantID("api")
	productionID := this.lookupEnvironmentID("production")

	repo := repository.NewDeploymentRepository(this.Pool)
	version, ok := repo.CurrentVersionInEnv(context.Background(), participantID, productionID)
	this.False(ok)
	this.Equal("", version)
}

func (this *IntegrationSuite) TestCurrentVersionInEnv_ScopedPerEnvironment() {
	this.seedApiParticipantContractAndProductionEnv()

	status, _ := this.post("/api/environments", stagingEnvBody)
	this.Require().Equal(http.StatusOK, status)

	participantID := this.lookupParticipantID("api")
	productionID := this.lookupEnvironmentID("production")
	stagingID := this.lookupEnvironmentID("staging")

	this.insertDeploymentAt(participantID, "v1", productionID, "2026-05-01T00:00:00Z")
	this.insertDeploymentAt(participantID, "v2", stagingID, "2026-05-02T00:00:00Z")

	repo := repository.NewDeploymentRepository(this.Pool)

	prodVersion, prodOk := repo.CurrentVersionInEnv(context.Background(), participantID, productionID)
	this.True(prodOk)
	this.Equal("v1", prodVersion)

	stagingVersion, stagingOk := repo.CurrentVersionInEnv(context.Background(), participantID, stagingID)
	this.True(stagingOk)
	this.Equal("v2", stagingVersion)
}
