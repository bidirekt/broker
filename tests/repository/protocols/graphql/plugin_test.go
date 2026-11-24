package graphql_test

import (
	"context"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/repository"
	graphqlrepository "github.com/bidirekt/broker/internal/repository/protocols/graphql"
	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	petArgumentsID = 101
	petReturnsID   = 102
	adoptPetID     = 103
	restResourceID = 104
	petAddedID     = 105
)

const seed = `
INSERT INTO participants (id, name) VALUES (1, 'pets_api');

INSERT INTO resources (id, participant_id, direction, protocol, body_role, provider_hash)
VALUES
  (101, 1, 'provides', 'graphql', 'input', 'query-pet-arguments'),
  (102, 1, 'provides', 'graphql', 'output', 'query-pet-returns'),
  (103, 1, 'provides', 'graphql', 'output', 'mutation-adopt-pet-returns'),
  (104, 1, 'provides', 'rest', 'output', 'get-pets-200'),
  (105, 1, 'provides', 'graphql', 'output', 'subscription-pet-added-returns');
`

type PluginSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
	plugin    graphqlrepository.Plugin
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

	this.plugin = graphqlrepository.New()
	this.plugin.Insert(ctx, pool, petArgumentsID, graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "arguments"})
	this.plugin.Insert(ctx, pool, petReturnsID, graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "returns"})
	this.plugin.Insert(ctx, pool, adoptPetID, graphqlmodel.Identity{Operation: "mutation", Field: "adoptPet", Interaction: "returns"})
}

func (this *PluginSuite) TearDownSuite() {
	if this.pool != nil {
		this.pool.Close()
	}
	if this.container != nil {
		_ = this.container.Terminate(context.Background())
	}
}

func (this *PluginSuite) TestProtocolIsGraphQL() {
	this.Equal(model.Protocol("graphql"), this.plugin.Protocol())
}

func (this *PluginSuite) TestInsertStoresOperationFieldAndInteraction() {
	type graphqlRow struct {
		Protocol    string
		Operation   string
		Field       string
		Interaction string
	}

	rows, err := this.pool.Query(context.Background(),
		`SELECT protocol, operation, field, interaction FROM graphql_resources WHERE resource_id = $1`, petArgumentsID)
	this.Require().NoError(err)
	stored, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[graphqlRow])
	this.Require().NoError(err)

	this.Equal(graphqlRow{Protocol: "graphql", Operation: "query", Field: "pet", Interaction: "arguments"}, stored)
}

func (this *PluginSuite) TestLoadIdentitiesReturnsOnlyTheRequestedResources() {
	identities := this.plugin.LoadIdentities(context.Background(), this.pool, []int64{petReturnsID, adoptPetID, 999})

	this.Equal(map[int64]model.ResourceIdentity{
		petReturnsID: graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "returns"},
		adoptPetID:   graphqlmodel.Identity{Operation: "mutation", Field: "adoptPet", Interaction: "returns"},
	}, identities)
}

func (this *PluginSuite) TestARESTResourceIsNeverAGraphQLRow() {
	this.Panics(func() {
		this.plugin.Insert(context.Background(), this.pool, restResourceID, graphqlmodel.Identity{Operation: "query", Field: "pets", Interaction: "returns"})
	})
}

func (this *PluginSuite) TestASubscriptionIsNeverAGraphQLRow() {
	this.Panics(func() {
		this.plugin.Insert(context.Background(), this.pool, petAddedID, graphqlmodel.Identity{Operation: "subscription", Field: "petAdded", Interaction: "returns"})
	})
}

func (this *PluginSuite) TestDoesNotHintTheProvider() {
	_, hints := any(this.plugin).(repository.ProviderHintTable)

	this.False(hints)
}
