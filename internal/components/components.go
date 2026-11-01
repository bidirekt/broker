package components

import (
	"context"
	"fmt"
	"time"

	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

const startupCleanupTimeout = 5 * time.Second

type Components struct {
	Server     *fiber.App
	Pool       *pgxpool.Pool
	ListenAddr string
	Settings   Settings
}

func New() (*Components, error) {
	settings, err := readSettings()
	if err != nil {
		return nil, err
	}

	pool, err := createDatabasePool()
	if err != nil {
		return nil, err
	}
	server := createHttpServer(settings, pool)

	if err := runMigrations(pool); err != nil {
		pool.Close()
		return nil, err
	}

	if err := deleteStaleSessionsAndLinks(pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Components{
		Server:     server,
		Pool:       pool,
		ListenAddr: ListenAddr(),
		Settings:   settings,
	}, nil
}

func deleteStaleSessionsAndLinks(pool *pgxpool.Pool) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), startupCleanupTimeout)
	defer cancel()

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("failed to delete stale sessions and links: %v", recovered)
		}
	}()

	sessions := repository.NewSessionRepository(pool)

	sessions.DeleteAdminSessions(ctx)
	sessions.DeleteExpired(ctx, sessionIdleLimit, sessionAbsoluteLimit)
	repository.NewOneTimeLinkRepository(pool).DeleteUsedAndExpired(ctx)

	return nil
}
