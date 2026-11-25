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

const (
	federatedContractID = 201
	plainContractID     = 202
	restContractID      = 203
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

INSERT INTO contracts (id, participant_id, checksum)
VALUES
  (201, 1, 'federated'),
  (202, 1, 'plain'),
  (203, 1, 'rest');
`

var federatedExtension = graphqlmodel.Extension{
	Graph: "petstore_graph",
	Directives: []graphqlmodel.Directive{
		{Coordinate: "Owner", Directive: "key", Fields: "id", Resolvable: false},
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet", Directive: "key", Fields: "name owner { id }", Resolvable: true},
		{Coordinate: "Pet.name", Directive: "shareable", Resolvable: true},
		{Coordinate: "Pet.rating", Directive: "override", From: "petstore_reviews", Resolvable: true},
	},
	Types: []graphqlmodel.TypeInfo{
		{Coordinate: "Owner", Kind: "object"},
		{Coordinate: "Owner.id", Kind: "field", NamedType: "ID"},
		{Coordinate: "Pet", Kind: "object"},
		{Coordinate: "Pet.id", Kind: "field", NamedType: "ID"},
		{Coordinate: "Pet.name", Kind: "field", NamedType: "String"},
		{Coordinate: "Pet.owner", Kind: "field", NamedType: "Owner"},
		{Coordinate: "Query.pet(id:)", Kind: "argument", NamedType: "ID"},
	},
	Roots: graphqlmodel.Roots{Query: "RootQuery", Mutation: "RootMutation"},
}

var plainExtension = graphqlmodel.Extension{
	Types: []graphqlmodel.TypeInfo{{Coordinate: "Query", Kind: "object"}},
}

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
	this.plugin.InsertContractExtension(ctx, pool, federatedContractID, federatedExtension)
	this.plugin.InsertContractExtension(ctx, pool, plainContractID, plainExtension)
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

func (this *PluginSuite) TestInsertContractExtensionStoresARowPerDirectiveAndPerTypeInfo() {
	type directiveRow struct {
		Coordinate string
		Directive  string
		Fields     string
		From       string
		Resolvable bool
	}

	rows, err := this.pool.Query(context.Background(),
		`SELECT coordinate, directive, fields, override_from, resolvable FROM graphql_federation_directives WHERE contract_id = $1 ORDER BY coordinate, fields`, federatedContractID)
	this.Require().NoError(err)
	directives, err := pgx.CollectRows(rows, pgx.RowToStructByPos[directiveRow])
	this.Require().NoError(err)

	var typeInfoRows int
	this.Require().NoError(this.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM graphql_type_info WHERE contract_id = $1`, federatedContractID).Scan(&typeInfoRows))

	this.Equal([]directiveRow{
		{Coordinate: "Owner", Directive: "key", Fields: "id", Resolvable: false},
		{Coordinate: "Pet", Directive: "key", Fields: "id", Resolvable: true},
		{Coordinate: "Pet", Directive: "key", Fields: "name owner { id }", Resolvable: true},
		{Coordinate: "Pet.name", Directive: "shareable", Fields: "", Resolvable: true},
		{Coordinate: "Pet.rating", Directive: "override", From: "petstore_reviews", Resolvable: true},
	}, directives)
	this.Equal(7, typeInfoRows)
}

func (this *PluginSuite) TestInsertContractExtensionStoresARowPerRootType() {
	type rootRow struct {
		ContractID int64
		Operation  string
		TypeName   string
	}

	rows, err := this.pool.Query(context.Background(),
		`SELECT contract_id, operation, type_name FROM graphql_root_types ORDER BY contract_id, operation`)
	this.Require().NoError(err)
	roots, err := pgx.CollectRows(rows, pgx.RowToStructByPos[rootRow])
	this.Require().NoError(err)

	this.Equal([]rootRow{
		{ContractID: federatedContractID, Operation: "mutation", TypeName: "RootMutation"},
		{ContractID: federatedContractID, Operation: "query", TypeName: "RootQuery"},
	}, roots)
}

func (this *PluginSuite) TestInsertContractExtensionStoresTheGraphOnlyWhenThereIsOne() {
	rows, err := this.pool.Query(context.Background(), `SELECT contract_id, graph_name FROM graphql_graph_members`)
	this.Require().NoError(err)
	members, err := pgx.CollectRows(rows, pgx.RowToStructByPos[struct {
		ContractID int64
		Graph      string
	}])
	this.Require().NoError(err)

	this.Len(members, 1)
	this.Equal(int64(federatedContractID), members[0].ContractID)
	this.Equal("petstore_graph", members[0].Graph)
}

func (this *PluginSuite) TestRefuseContractExtensionRefusesAGraphNamedAfterAParticipant() {
	refusal, refused := this.plugin.RefuseContractExtension(context.Background(), this.pool,
		graphqlmodel.Extension{Graph: "pets_api", GraphSource: "graph.yaml"})

	this.True(refused)
	this.Equal(model.Refusal{
		Code:    "graph.name_taken",
		Path:    "provides;graphql;graph",
		Source:  "graph.yaml",
		Details: map[string]any{"graph": "pets_api"},
	}, refusal)
}

func (this *PluginSuite) TestRefuseContractExtensionTakesAFreeGraphNameAndNoGraph() {
	_, refusedFree := this.plugin.RefuseContractExtension(context.Background(), this.pool,
		graphqlmodel.Extension{Graph: "petstore_graph", GraphSource: "graph.yaml"})
	_, refusedWithout := this.plugin.RefuseContractExtension(context.Background(), this.pool, plainExtension)

	this.False(refusedFree)
	this.False(refusedWithout)
}

func (this *PluginSuite) TestLoadContractExtensionsReturnsWhatWasInsertedForTheRequestedContracts() {
	extensions := this.plugin.LoadContractExtensions(context.Background(), this.pool, []int64{federatedContractID, plainContractID, restContractID})

	this.Equal(map[int64]any{
		federatedContractID: federatedExtension,
		plainContractID:     plainExtension,
	}, extensions)
}

func (this *PluginSuite) TestADirectiveThatDoesNotCountIsNeverStored() {
	this.Panics(func() {
		this.plugin.InsertContractExtension(context.Background(), this.pool, restContractID, graphqlmodel.Extension{
			Directives: []graphqlmodel.Directive{{Coordinate: "Pet.weight", Directive: "requires", Resolvable: true}},
		})
	})
}

func (this *PluginSuite) TestReservedParticipantNameReservesAGraphName() {
	message, reserved := this.plugin.ReservedParticipantName(context.Background(), this.pool, "petstore_graph")
	_, freeReserved := this.plugin.ReservedParticipantName(context.Background(), this.pool, "petstore_web")

	this.True(reserved)
	this.Equal(`"petstore_graph" is a graph name; pick another participant name`, message)
	this.False(freeReserved)
}
