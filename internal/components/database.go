package components

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultDatabaseConnectTimeout = 5 * time.Second
	databaseConnectRetryPause     = 2 * time.Second
)

func createDatabasePool() (*pgxpool.Pool, error) {
	url, err := databaseURL()
	if err != nil {
		return nil, err
	}

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("invalid BIDIREKT_DATABASE_URL: %w", err)
	}

	attempts, err := databaseConnectAttempts()
	if err != nil {
		return nil, err
	}

	// a ping that gives up does not stop the dial: pgxpool keeps it going in the
	// background, for 2 minutes when the URL sets no connect_timeout
	if config.ConnConfig.ConnectTimeout == 0 {
		config.ConnConfig.ConnectTimeout = defaultDatabaseConnectTimeout
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			time.Sleep(databaseConnectRetryPause)
		}

		if err = pingDatabase(pool, config.ConnConfig.ConnectTimeout); err == nil {
			return pool, nil
		}

		log.Printf("database not ready (attempt %d/%d): %v", attempt, attempts, err)
	}

	pool.Close()

	return nil, fmt.Errorf("database not ready, giving up after attempt %d: %w", attempts, err)
}

func pingDatabase(pool *pgxpool.Pool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return pool.Ping(ctx)
}

func runMigrations(pool *pgxpool.Pool) error {
	m := migrator.New(pool, migrations.FS, "public.schema_migrations")

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
