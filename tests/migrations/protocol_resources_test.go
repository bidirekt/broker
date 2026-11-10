package migrations_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	protocolResourcesMigration = "20261109120000_protocol_resources.sql"
	migrationsTable            = "public.schema_migrations"

	requestResourceID  = 41
	responseResourceID = 42

	notNullViolation    = "23502"
	foreignKeyViolation = "23503"
	checkViolation      = "23514"
)

const oldFormatData = `
INSERT INTO participants (id, name) VALUES (1, 'pet-api'), (2, 'pet-web');
INSERT INTO contracts (id, participant_id, checksum) VALUES (10, 1, 'provider-checksum'), (20, 2, 'consumer-checksum');

INSERT INTO resources (id, participant_id, direction, interaction, consumed_provider, endpoint, method, response_status_code, provider_hash, consumer_hash)
VALUES
  (41, 2, 'consumes', 'rest_request', 'pet-api', '/pets', 'post', NULL, 'provider-hash-post', 'consumer-hash-post'),
  (42, 1, 'provides', 'rest_response', NULL, '/pets', 'get', '200', 'provider-hash-get', NULL);

INSERT INTO compatibility_verdicts (contract_id_one, contract_id_two, breaks)
VALUES
  (10, 20, '[
    {"endpoint": "/pets", "method": "get", "interaction": "200", "reason": "property_missing_in_provider", "details": {"property": "name"}},
    {"endpoint": "/pets", "method": "post", "interaction": "request", "reason": "property_type_mismatch", "details": {"property": "age", "consumerPropertyType": "integer", "providerPropertyType": "string"}}
  ]'),
  (10, 10, '[]');
`

type ProtocolResourcesSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
}

func TestProtocolResourcesSuite(t *testing.T) {
	suite.Run(t, new(ProtocolResourcesSuite))
}

func (this *ProtocolResourcesSuite) SetupSuite() {
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

	this.Require().NoError(migrator.New(pool, this.migrationsBefore(protocolResourcesMigration), migrationsTable).Migrate())
	_, err = pool.Exec(ctx, oldFormatData)
	this.Require().NoError(err)
	this.Require().NoError(migrator.New(pool, migrations.FS, migrationsTable).Migrate())
}

func (this *ProtocolResourcesSuite) TearDownSuite() {
	if this.pool != nil {
		this.pool.Close()
	}
	if this.container != nil {
		_ = this.container.Terminate(context.Background())
	}
}

func (this *ProtocolResourcesSuite) migrationsBefore(name string) fs.FS {
	entries, err := fs.ReadDir(migrations.FS, ".")
	this.Require().NoError(err)

	earlier := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Name() >= name {
			continue
		}
		contents, err := fs.ReadFile(migrations.FS, entry.Name())
		this.Require().NoError(err)
		earlier[entry.Name()] = &fstest.MapFile{Data: contents}
	}
	return earlier
}

func (this *ProtocolResourcesSuite) execError(sql string, args ...any) string {
	ctx := context.Background()
	tx, err := this.pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, sql, args...)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		this.Failf("expected a database error", "got %v", err)
		return ""
	}
	return pgErr.Code
}

func (this *ProtocolResourcesSuite) TestResourcesKeepTheirIDsAndBecomeRestWithABodyRole() {
	type resourceRow struct {
		ID       int64
		Protocol string
		BodyRole string
	}

	rows, err := this.pool.Query(context.Background(), `SELECT id, protocol, body_role FROM resources ORDER BY id`)
	this.Require().NoError(err)
	resources, err := pgx.CollectRows(rows, pgx.RowToStructByPos[resourceRow])
	this.Require().NoError(err)

	this.Equal([]resourceRow{
		{ID: requestResourceID, Protocol: "rest", BodyRole: "input"},
		{ID: responseResourceID, Protocol: "rest", BodyRole: "output"},
	}, resources)
}

func (this *ProtocolResourcesSuite) TestEachResourceHasOneRestRowWithItsEndpointMethodAndStatus() {
	type restRow struct {
		ResourceID         int64
		Protocol           string
		Endpoint           string
		Method             string
		ResponseStatusCode *string
	}

	rows, err := this.pool.Query(context.Background(),
		`SELECT resource_id, protocol, endpoint, method, response_status_code FROM rest_resources ORDER BY resource_id`)
	this.Require().NoError(err)
	restResources, err := pgx.CollectRows(rows, pgx.RowToStructByPos[restRow])
	this.Require().NoError(err)

	status := "200"
	this.Equal([]restRow{
		{ResourceID: requestResourceID, Protocol: "rest", Endpoint: "/pets", Method: "post", ResponseStatusCode: nil},
		{ResourceID: responseResourceID, Protocol: "rest", Endpoint: "/pets", Method: "get", ResponseStatusCode: &status},
	}, restResources)
}

func (this *ProtocolResourcesSuite) TestStoredBreaksNameTheirProtocol() {
	var breaks string
	var deployable bool
	err := this.pool.QueryRow(context.Background(),
		`SELECT breaks::text, deployable FROM compatibility_verdicts WHERE contract_id_one = 10 AND contract_id_two = 20`,
	).Scan(&breaks, &deployable)
	this.Require().NoError(err)

	this.JSONEq(`[
		{"rest": {"endpoint": "/pets", "method": "get", "interaction": "200"}, "reason": "property_missing_in_provider", "details": {"property": "name"}},
		{"rest": {"endpoint": "/pets", "method": "post", "interaction": "request"}, "reason": "property_type_mismatch", "details": {"property": "age", "consumerPropertyType": "integer", "providerPropertyType": "string"}}
	]`, breaks)
	this.False(deployable)
}

func (this *ProtocolResourcesSuite) TestAVerdictWithoutBreaksStaysEmptyAndDeployable() {
	var breaks string
	var deployable bool
	err := this.pool.QueryRow(context.Background(),
		`SELECT breaks::text, deployable FROM compatibility_verdicts WHERE contract_id_one = 10 AND contract_id_two = 10`,
	).Scan(&breaks, &deployable)
	this.Require().NoError(err)

	this.JSONEq(`[]`, breaks)
	this.True(deployable)
}

func (this *ProtocolResourcesSuite) TestRestColumnsLeftResources() {
	rows, err := this.pool.Query(context.Background(),
		`SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'resources' ORDER BY ordinal_position`)
	this.Require().NoError(err)
	columns, err := pgx.CollectRows(rows, pgx.RowTo[string])
	this.Require().NoError(err)

	this.Equal([]string{
		"id", "participant_id", "direction", "consumed_provider", "provider_hash", "consumer_hash", "created_at", "protocol", "body_role",
	}, columns)
}

func (this *ProtocolResourcesSuite) TestARestRowCannotPointAtAResourceOfAnotherProtocol() {
	ctx := context.Background()
	tx, err := this.pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO resources (id, participant_id, direction, protocol, body_role, provider_hash)
		VALUES (50, 1, 'provides', 'grpc', 'output', 'provider-hash-grpc')`)
	this.Require().NoError(err)

	_, err = tx.Exec(ctx, `INSERT INTO rest_resources (resource_id, protocol, endpoint, method) VALUES (50, 'rest', '/pets', 'get')`)
	var pgErr *pgconn.PgError
	this.Require().ErrorAs(err, &pgErr)
	this.Equal(foreignKeyViolation, pgErr.Code)
}

func (this *ProtocolResourcesSuite) TestAResourceWithoutProtocolIsRefused() {
	this.Equal(notNullViolation, this.execError(`
		INSERT INTO resources (participant_id, direction, protocol, body_role, provider_hash)
		VALUES (1, 'provides', NULL, 'output', 'provider-hash-null-protocol')`))
}

func (this *ProtocolResourcesSuite) TestAResourceWithoutBodyRoleIsRefused() {
	this.Equal(notNullViolation, this.execError(`
		INSERT INTO resources (participant_id, direction, protocol, body_role, provider_hash)
		VALUES (1, 'provides', 'rest', NULL, 'provider-hash-null-body-role')`))
}

func (this *ProtocolResourcesSuite) TestARestRowWithoutProtocolIsRefused() {
	this.Equal(notNullViolation, this.execError(
		`INSERT INTO rest_resources (resource_id, protocol, endpoint, method) VALUES (999, NULL, '/pets', 'get')`))
}

func (this *ProtocolResourcesSuite) TestAnUnknownBodyRoleIsRefused() {
	this.Equal(checkViolation, this.execError(`
		INSERT INTO resources (participant_id, direction, protocol, body_role, provider_hash)
		VALUES (1, 'provides', 'rest', 'x', 'provider-hash-unknown-body-role')`))
}

func (this *ProtocolResourcesSuite) TestAnUnknownDirectionIsRefused() {
	this.Equal(checkViolation, this.execError(`
		INSERT INTO resources (participant_id, direction, protocol, body_role, provider_hash, consumer_hash)
		VALUES (1, 'x', 'rest', 'input', 'provider-hash-unknown-direction', 'consumer-hash-unknown-direction')`))
}
