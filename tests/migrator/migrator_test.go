package migrator_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type MigratorSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
	dir       string
}

func TestMigratorSuite(t *testing.T) {
	suite.Run(t, new(MigratorSuite))
}

func (this *MigratorSuite) SetupTest() {
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
	if err != nil {
		this.T().Fatalf("Failed to run postgres container: %v", err)
	}
	this.container = container

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		this.T().Fatalf("Failed to get postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		this.T().Fatalf("Failed to create database pool: %v", err)
	}
	this.pool = pool

	this.dir = this.T().TempDir()
}

func (this *MigratorSuite) TearDownTest() {
	if this.pool != nil {
		this.pool.Close()
	}
	if this.container != nil {
		_ = this.container.Terminate(context.Background())
	}
}

func (this *MigratorSuite) writeMigration(name, sql string) {
	path := filepath.Join(this.dir, name)
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		this.T().Fatalf("Failed to write migration file: %v", err)
	}
}

func (this *MigratorSuite) countMigrations() int {
	var count int
	if err := this.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM public.schema_migrations").Scan(&count); err != nil {
		this.T().Fatalf("Failed to count migrations: %v", err)
	}
	return count
}

func (this *MigratorSuite) appliedMigrations() []string {
	rows, err := this.pool.Query(context.Background(), "SELECT migration FROM public.schema_migrations ORDER BY id")
	if err != nil {
		this.T().Fatalf("Failed to query migrations: %v", err)
	}
	migrations, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		this.T().Fatalf("Failed to collect migrations: %v", err)
	}
	return migrations
}

func (this *MigratorSuite) schemaExists(name string) bool {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`
	if err := this.pool.QueryRow(context.Background(), query, name).Scan(&exists); err != nil {
		this.T().Fatalf("Failed to check schema existence: %v", err)
	}
	return exists
}

func (this *MigratorSuite) tableExists(name string) bool {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`
	if err := this.pool.QueryRow(context.Background(), query, name).Scan(&exists); err != nil {
		this.T().Fatalf("Failed to check table existence: %v", err)
	}
	return exists
}

func (this *MigratorSuite) TestAppliesAllPendingMigrations() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	this.writeMigration("20260102000000_create_bar.sql", "CREATE TABLE bar (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	if err := m.Migrate(); err != nil {
		this.T().Fatalf("Migrate returned error: %v", err)
	}

	this.Equal(2, this.countMigrations())
	this.True(this.tableExists("foo"))
	this.True(this.tableExists("bar"))
}

func (this *MigratorSuite) TestRecordsTheBareFileNameAsMigrationKey() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	this.writeMigration("20260102000000_create_bar.sql", "CREATE TABLE bar (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.NoError(m.Migrate())

	this.Equal(
		[]string{"20260101000000_create_foo.sql", "20260102000000_create_bar.sql"},
		this.appliedMigrations(),
	)
}

func (this *MigratorSuite) TestIsIdempotent() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.NoError(m.Migrate())
	this.NoError(m.Migrate())

	this.Equal(1, this.countMigrations())
}

func (this *MigratorSuite) TestPicksUpNewlyAddedMigrations() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.NoError(m.Migrate())
	this.Equal(1, this.countMigrations())

	this.writeMigration("20260102000000_create_bar.sql", "CREATE TABLE bar (id SERIAL PRIMARY KEY);")
	this.NoError(m.Migrate())

	this.Equal(2, this.countMigrations())
	this.True(this.tableExists("bar"))
}

func (this *MigratorSuite) TestEnsuresMigrationsTable() {
	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.NoError(m.Migrate())

	this.True(this.tableExists("schema_migrations"))
	this.Equal(0, this.countMigrations())
}

func (this *MigratorSuite) TestUnqualifiedTableNameCreatesNoSchema() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "schema_migrations")
	this.NoError(m.Migrate())

	// the table resolves through search_path into public, and the table name is not
	// mistaken for a schema to create
	this.True(this.tableExists("schema_migrations"))
	this.False(this.schemaExists("schema_migrations"))
	this.Equal(1, this.countMigrations())
}

func (this *MigratorSuite) TestQualifiedTableNameCreatesItsSchema() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "meta.schema_migrations")
	this.NoError(m.Migrate())

	this.True(this.schemaExists("meta"))

	var count int
	this.Require().NoError(this.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM meta.schema_migrations").Scan(&count))
	this.Equal(1, count)
}

func (this *MigratorSuite) TestSkipsNonSqlAndDirectories() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	if err := os.WriteFile(filepath.Join(this.dir, "README.md"), []byte("ignore me"), 0644); err != nil {
		this.T().Fatalf("Failed to write README: %v", err)
	}
	if err := os.Mkdir(filepath.Join(this.dir, "subdir"), 0755); err != nil {
		this.T().Fatalf("Failed to create subdir: %v", err)
	}

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.NoError(m.Migrate())

	this.Equal(1, this.countMigrations())
}

func (this *MigratorSuite) TestReturnsErrorOnInvalidSQL() {
	this.writeMigration("20260101000000_broken.sql", "THIS IS NOT VALID SQL;")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	err := m.Migrate()

	this.Error(err)
	this.Equal(0, this.countMigrations())
}

func (this *MigratorSuite) TestRollsBackAllPendingWhenAnyMigrationFails() {
	this.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	this.writeMigration("20260102000000_broken.sql", "THIS IS NOT VALID SQL;")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	err := m.Migrate()

	this.Error(err)
	this.Equal(0, this.countMigrations())
	this.False(this.tableExists("foo"))
}

func (this *MigratorSuite) TestPanicsOnInvalidFilenameFormat() {
	this.writeMigration("0001_legacy_format.sql", "CREATE TABLE legacy (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.PanicsWithValue(
		`migrator: invalid migration filename "0001_legacy_format.sql": expected format YYYYMMDDHHMMSS_subject.sql (e.g. 20260520143022_add_users_table.sql)`,
		func() { _ = m.Migrate() },
	)
}

func (this *MigratorSuite) TestPanicsOnInvalidTimestamp() {
	this.writeMigration("20260230000000_bad_date.sql", "CREATE TABLE bad (id SERIAL PRIMARY KEY);")

	m := migrator.New(this.pool, os.DirFS(this.dir), "public.schema_migrations")
	this.Panics(func() { _ = m.Migrate() })
}
