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

func (s *MigratorSuite) SetupTest() {
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
		s.T().Fatalf("Failed to run postgres container: %v", err)
	}
	s.container = container

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		s.T().Fatalf("Failed to get postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		s.T().Fatalf("Failed to create database pool: %v", err)
	}
	s.pool = pool

	s.dir = s.T().TempDir()
}

func (s *MigratorSuite) TearDownTest() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *MigratorSuite) writeMigration(name, sql string) {
	path := filepath.Join(s.dir, name)
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		s.T().Fatalf("Failed to write migration file: %v", err)
	}
}

func (s *MigratorSuite) countMigrations() int {
	var count int
	if err := s.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM public.schema_migrations").Scan(&count); err != nil {
		s.T().Fatalf("Failed to count migrations: %v", err)
	}
	return count
}

func (s *MigratorSuite) appliedMigrations() []string {
	rows, err := s.pool.Query(context.Background(), "SELECT migration FROM public.schema_migrations ORDER BY id")
	if err != nil {
		s.T().Fatalf("Failed to query migrations: %v", err)
	}
	migrations, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		s.T().Fatalf("Failed to collect migrations: %v", err)
	}
	return migrations
}

func (s *MigratorSuite) schemaExists(name string) bool {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`
	if err := s.pool.QueryRow(context.Background(), query, name).Scan(&exists); err != nil {
		s.T().Fatalf("Failed to check schema existence: %v", err)
	}
	return exists
}

func (s *MigratorSuite) tableExists(name string) bool {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`
	if err := s.pool.QueryRow(context.Background(), query, name).Scan(&exists); err != nil {
		s.T().Fatalf("Failed to check table existence: %v", err)
	}
	return exists
}

func (s *MigratorSuite) TestAppliesAllPendingMigrations() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	s.writeMigration("20260102000000_create_bar.sql", "CREATE TABLE bar (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	if err := m.Migrate(); err != nil {
		s.T().Fatalf("Migrate returned error: %v", err)
	}

	s.Equal(2, s.countMigrations())
	s.True(s.tableExists("foo"))
	s.True(s.tableExists("bar"))
}

func (s *MigratorSuite) TestRecordsTheBareFileNameAsMigrationKey() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	s.writeMigration("20260102000000_create_bar.sql", "CREATE TABLE bar (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.NoError(m.Migrate())

	s.Equal(
		[]string{"20260101000000_create_foo.sql", "20260102000000_create_bar.sql"},
		s.appliedMigrations(),
	)
}

func (s *MigratorSuite) TestIsIdempotent() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.NoError(m.Migrate())
	s.NoError(m.Migrate())

	s.Equal(1, s.countMigrations())
}

func (s *MigratorSuite) TestPicksUpNewlyAddedMigrations() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.NoError(m.Migrate())
	s.Equal(1, s.countMigrations())

	s.writeMigration("20260102000000_create_bar.sql", "CREATE TABLE bar (id SERIAL PRIMARY KEY);")
	s.NoError(m.Migrate())

	s.Equal(2, s.countMigrations())
	s.True(s.tableExists("bar"))
}

func (s *MigratorSuite) TestEnsuresMigrationsTable() {
	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.NoError(m.Migrate())

	s.True(s.tableExists("schema_migrations"))
	s.Equal(0, s.countMigrations())
}

func (s *MigratorSuite) TestUnqualifiedTableNameCreatesNoSchema() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "schema_migrations")
	s.NoError(m.Migrate())

	// the table resolves through search_path into public, and the table name is not
	// mistaken for a schema to create
	s.True(s.tableExists("schema_migrations"))
	s.False(s.schemaExists("schema_migrations"))
	s.Equal(1, s.countMigrations())
}

func (s *MigratorSuite) TestQualifiedTableNameCreatesItsSchema() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "meta.schema_migrations")
	s.NoError(m.Migrate())

	s.True(s.schemaExists("meta"))

	var count int
	s.Require().NoError(s.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM meta.schema_migrations").Scan(&count))
	s.Equal(1, count)
}

func (s *MigratorSuite) TestSkipsNonSqlAndDirectories() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	if err := os.WriteFile(filepath.Join(s.dir, "README.md"), []byte("ignore me"), 0644); err != nil {
		s.T().Fatalf("Failed to write README: %v", err)
	}
	if err := os.Mkdir(filepath.Join(s.dir, "subdir"), 0755); err != nil {
		s.T().Fatalf("Failed to create subdir: %v", err)
	}

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.NoError(m.Migrate())

	s.Equal(1, s.countMigrations())
}

func (s *MigratorSuite) TestReturnsErrorOnInvalidSQL() {
	s.writeMigration("20260101000000_broken.sql", "THIS IS NOT VALID SQL;")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	err := m.Migrate()

	s.Error(err)
	s.Equal(0, s.countMigrations())
}

func (s *MigratorSuite) TestRollsBackAllPendingWhenAnyMigrationFails() {
	s.writeMigration("20260101000000_create_foo.sql", "CREATE TABLE foo (id SERIAL PRIMARY KEY);")
	s.writeMigration("20260102000000_broken.sql", "THIS IS NOT VALID SQL;")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	err := m.Migrate()

	s.Error(err)
	s.Equal(0, s.countMigrations())
	s.False(s.tableExists("foo"))
}

func (s *MigratorSuite) TestPanicsOnInvalidFilenameFormat() {
	s.writeMigration("0001_legacy_format.sql", "CREATE TABLE legacy (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.PanicsWithValue(
		`migrator: invalid migration filename "0001_legacy_format.sql": expected format YYYYMMDDHHMMSS_subject.sql (e.g. 20260520143022_add_users_table.sql)`,
		func() { _ = m.Migrate() },
	)
}

func (s *MigratorSuite) TestPanicsOnInvalidTimestamp() {
	s.writeMigration("20260230000000_bad_date.sql", "CREATE TABLE bad (id SERIAL PRIMARY KEY);")

	m := migrator.New(s.pool, os.DirFS(s.dir), "public.schema_migrations")
	s.Panics(func() { _ = m.Migrate() })
}
