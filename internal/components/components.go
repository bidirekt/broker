package components

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Components struct {
	Server     *fiber.App
	Pool       *pgxpool.Pool
	ListenAddr string
}

const defaultListenAddr = ":8080"

func ListenAddr() string {
	if listenAddr := os.Getenv("BIDIREKT_LISTEN_ADDR"); listenAddr != "" {
		return listenAddr
	}

	return defaultListenAddr
}

const (
	defaultDatabaseConnectAttempts = 10
	defaultDatabaseConnectTimeout  = 5 * time.Second
	databaseConnectRetryPause      = 2 * time.Second
)

func databaseConnectAttempts() (int, error) {
	value := os.Getenv("BIDIREKT_DATABASE_CONNECT_RETRIES")
	if value == "" {
		return defaultDatabaseConnectAttempts, nil
	}

	attempts, err := strconv.Atoi(value)
	if err != nil || attempts < 1 {
		return 0, errors.New("BIDIREKT_DATABASE_CONNECT_RETRIES must be a positive integer")
	}

	return attempts, nil
}

func createDatabasePool() (*pgxpool.Pool, error) {
	databaseURL := os.Getenv("BIDIREKT_DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("BIDIREKT_DATABASE_URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
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

// contracts of a few thousand resources do not fit fiber's 4 MiB default
const publishBodyLimit = 8 * 1024 * 1024

const internalErrorMessage = "internal error"

type errorResponseBody struct {
	Message string `json:"message"`
}

func respondError(ctx fiber.Ctx, err error) error {
	log.Printf("request %s %s failed: %v", ctx.Method(), ctx.Path(), err)

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return ctx.Status(fiberErr.Code).JSON(errorResponseBody{Message: fiberErr.Message})
	}

	return ctx.Status(fiber.StatusInternalServerError).JSON(errorResponseBody{Message: internalErrorMessage})
}

func createHttpServer() *fiber.App {
	server := fiber.New(fiber.Config{
		BodyLimit:    publishBodyLimit,
		ErrorHandler: respondError,
	})

	server.Use(recover.New(recover.Config{EnableStackTrace: true}))

	return server
}

func runMigrations(pool *pgxpool.Pool) error {
	m := migrator.New(pool, migrations.FS, "public.schema_migrations")

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func New() (*Components, error) {
	pool, err := createDatabasePool()
	if err != nil {
		return nil, err
	}
	server := createHttpServer()

	if err := runMigrations(pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Components{
		Server:     server,
		Pool:       pool,
		ListenAddr: ListenAddr(),
	}, nil
}
