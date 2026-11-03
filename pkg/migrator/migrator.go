package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationTimestampLayout = "20060102150405"

var migrationFilenamePattern = regexp.MustCompile(`^(\d{14})_([a-z0-9]+(?:_[a-z0-9]+)*)\.sql$`)

const ensureMigrationsSchemaSQL = `
CREATE SCHEMA IF NOT EXISTS %s;
`

const ensureMigrationsTableSQL = `
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    migration VARCHAR(255) NOT NULL UNIQUE,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`

const lockMigrationsSQL = `SELECT pg_advisory_xact_lock(hashtext($1));`

const selectMigrationSQL = `SELECT COUNT(*) FROM %s WHERE migration = $1;`

const insertMigrationSQL = `INSERT INTO %s (migration) VALUES ($1);`

type Migrator struct {
	pool            *pgxpool.Pool
	migrations      fs.FS
	migrationsTable string
}

func New(
	pool *pgxpool.Pool,
	migrations fs.FS,
	migrationsTable string,
) *Migrator {
	return &Migrator{
		pool:            pool,
		migrations:      migrations,
		migrationsTable: migrationsTable,
	}
}

func (this *Migrator) Migrate() error {
	migrationFiles, err := this.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("error reading migrations directory: %w", err)
	}

	ctx := context.Background()
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error starting migrations transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, lockMigrationsSQL, this.migrationsTable); err != nil {
		return fmt.Errorf("error locking migrations: %w", err)
	}

	if err := this.ensureMigrationsTable(ctx, tx); err != nil {
		return fmt.Errorf("error ensuring migrations table: %w", err)
	}

	for _, migrationFile := range migrationFiles {
		applied, err := this.isMigrationApplied(ctx, tx, migrationFile)
		if err != nil {
			return fmt.Errorf(
				"error checking if migration %s is applied: %w",
				migrationFile,
				err,
			)
		}

		if applied {
			continue
		}

		if err := this.applyMigration(ctx, tx, migrationFile); err != nil {
			return err
		}

		log.Printf("migrated %s", migrationFile)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("error committing migrations: %w", err)
	}

	return nil
}

func (this *Migrator) ensureMigrationsTable(ctx context.Context, tx pgx.Tx) error {
	var sql string

	// only a qualified name identifies a schema to create; an unqualified one resolves
	// through search_path and must not be mistaken for a schema of its own
	if schema, _, qualified := strings.Cut(this.migrationsTable, "."); qualified {
		sql = fmt.Sprintf(ensureMigrationsSchemaSQL, schema)
	}

	sql += fmt.Sprintf(ensureMigrationsTableSQL, this.migrationsTable)

	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}

	return nil
}

func (this *Migrator) getMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(this.migrations, ".")
	if err != nil {
		return nil, err
	}

	var migrationFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		assertValidMigrationFilename(entry.Name())
		migrationFiles = append(migrationFiles, entry.Name())
	}

	return migrationFiles, nil
}

func assertValidMigrationFilename(name string) {
	matches := migrationFilenamePattern.FindStringSubmatch(name)
	if matches == nil {
		panic(fmt.Sprintf(
			"migrator: invalid migration filename %q: expected format YYYYMMDDHHMMSS_subject.sql (e.g. 20260520143022_add_users_table.sql)",
			name,
		))
	}
	if _, err := time.Parse(migrationTimestampLayout, matches[1]); err != nil {
		panic(fmt.Sprintf(
			"migrator: invalid migration timestamp in %q: %v",
			name, err,
		))
	}
}

func (this *Migrator) isMigrationApplied(ctx context.Context, tx pgx.Tx, migrationFile string) (bool, error) {
	sql := fmt.Sprintf(selectMigrationSQL, this.migrationsTable)

	var count int
	if err := tx.QueryRow(ctx, sql, migrationFile).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func (this *Migrator) applyMigration(ctx context.Context, tx pgx.Tx, migrationFile string) error {
	contents, err := fs.ReadFile(this.migrations, migrationFile)
	if err != nil {
		return fmt.Errorf("error reading migration file %s: %w", migrationFile, err)
	}

	if _, err := tx.Exec(ctx, string(contents)); err != nil {
		return fmt.Errorf("error executing migration %s: %w", migrationFile, err)
	}

	sql := fmt.Sprintf(insertMigrationSQL, this.migrationsTable)
	if _, err := tx.Exec(ctx, sql, migrationFile); err != nil {
		return fmt.Errorf("error recording migration %s: %w", migrationFile, err)
	}

	return nil
}
