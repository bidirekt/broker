package rest_test

import (
	"context"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	deployedEnvironmentID = 1
	emptyEnvironmentID    = 2

	getPetsOkID       = 101
	getPetsNotFoundID = 102
	postPetsRequestID = 103
	getPetsRemovedID  = 104
)

const seed = `
INSERT INTO participants (id, name) VALUES (1, 'pet-api');
INSERT INTO environments (id, name) VALUES (1, 'production'), (2, 'staging');
INSERT INTO contracts (id, participant_id, checksum) VALUES (9, 1, 'first'), (10, 1, 'second');
INSERT INTO contract_versions (participant_id, version, contract_id, contract_content) VALUES (1, '1', 9, ''), (1, '2', 10, '');
INSERT INTO current_deployments (participant_id, environment_id, version, contract_id, deployed_at) VALUES (1, 1, '2', 10, now());

INSERT INTO resources (id, participant_id, direction, protocol, body_role, provider_hash)
VALUES
  (101, 1, 'provides', 'rest', 'output', 'get-pets-200'),
  (102, 1, 'provides', 'rest', 'output', 'get-pets-404'),
  (103, 1, 'provides', 'rest', 'input', 'post-pets-request'),
  (104, 1, 'provides', 'rest', 'output', 'get-pets-500');

INSERT INTO resource_versions (resource_id, contract_id, change_type)
VALUES
  (101, 9, 'added'),
  (102, 9, 'added'),
  (103, 9, 'added'),
  (104, 9, 'added'),
  (104, 10, 'removed');
`

type PluginSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
	plugin    restrepository.Plugin
}

func TestPluginSuite(t *testing.T) {
	suite.Run(t, new(PluginSuite))
}

func (this *PluginSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(
		ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("contracttests"),
		postgres.WithUsername("contracttests"),
		postgres.WithPassword("s3cr3t"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	this.Require().NoError(err)
	this.container = container

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	this.Require().NoError(err)

	pool, err := pgxpool.New(ctx, connectionString)
	this.Require().NoError(err)
	this.pool = pool

	this.Require().NoError(migrator.New(pool, migrations.FS, "public.schema_migrations").Migrate())
	_, err = pool.Exec(ctx, seed)
	this.Require().NoError(err)

	this.plugin = restrepository.New()
	this.plugin.Insert(ctx, pool, getPetsOkID, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"})
	this.plugin.Insert(ctx, pool, getPetsNotFoundID, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "404"})
	this.plugin.Insert(ctx, pool, postPetsRequestID, restmodel.Identity{Endpoint: "/pets", Method: "post"})
	this.plugin.Insert(ctx, pool, getPetsRemovedID, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "500"})
}

func (this *PluginSuite) TearDownSuite() {
	if this.pool != nil {
		this.pool.Close()
	}
	if this.container != nil {
		_ = this.container.Terminate(context.Background())
	}
}

func (this *PluginSuite) consumer(method, status string) model.PersistedResource {
	return model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  "pet-web",
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             model.Output,
			Identity:         restmodel.Identity{Endpoint: "/pets", Method: method, Status: status},
			ConsumedProvider: null.StringFrom("pet-api"),
		},
	}
}

func (this *PluginSuite) TestProtocolIsRest() {
	this.Equal(model.Protocol("rest"), this.plugin.Protocol())
}

func (this *PluginSuite) TestInsertStoresTheRequestWithoutAStatus() {
	type restRow struct {
		Protocol           string
		Endpoint           string
		Method             string
		ResponseStatusCode *string
	}

	rows, err := this.pool.Query(context.Background(),
		`SELECT protocol, endpoint, method, response_status_code FROM rest_resources WHERE resource_id = $1`, postPetsRequestID)
	this.Require().NoError(err)
	stored, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[restRow])
	this.Require().NoError(err)

	this.Equal(restRow{Protocol: "rest", Endpoint: "/pets", Method: "post", ResponseStatusCode: nil}, stored)
}

func (this *PluginSuite) TestLoadIdentitiesReturnsOnlyTheRequestedResources() {
	identities := this.plugin.LoadIdentities(context.Background(), this.pool, []int64{getPetsOkID, postPetsRequestID, 999})

	this.Equal(map[int64]model.ResourceIdentity{
		getPetsOkID:       restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"},
		postPetsRequestID: restmodel.Identity{Endpoint: "/pets", Method: "post", Status: ""},
	}, identities)
}

func (this *PluginSuite) TestProviderHintListsTheLiveStatusesOfTheDeployedMethod() {
	hint, found := this.plugin.LoadProviderHint(context.Background(), this.pool, this.consumer("get", "201"), deployedEnvironmentID)

	this.True(found)
	this.Equal(restmodel.DeployedStatuses{"200", "404"}, hint)
}

func (this *PluginSuite) TestProviderHintOfAMethodWithOnlyARequestHasNoStatuses() {
	hint, found := this.plugin.LoadProviderHint(context.Background(), this.pool, this.consumer("post", "201"), deployedEnvironmentID)

	this.True(found)
	this.Empty(hint)
	this.IsType(restmodel.DeployedStatuses{}, hint)
}

func (this *PluginSuite) TestProviderHintIsNotFoundForAMethodTheProviderLacks() {
	_, found := this.plugin.LoadProviderHint(context.Background(), this.pool, this.consumer("delete", "204"), deployedEnvironmentID)

	this.False(found)
}

func (this *PluginSuite) TestProviderHintIsNotFoundWhereTheProviderIsNotDeployed() {
	_, found := this.plugin.LoadProviderHint(context.Background(), this.pool, this.consumer("get", "201"), emptyEnvironmentID)

	this.False(found)
}
