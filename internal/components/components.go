package components

import (
	"context"
	"fmt"
	"time"

	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/database"
	"github.com/bidirekt/broker/internal/components/server"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

const startupCleanupTimeout = 5 * time.Second

type Components struct {
	Server     *fiber.App
	Pool       *pgxpool.Pool
	ListenAddr string
	Settings   config.Settings
}

func New() (*Components, error) {
	settings, err := config.Read()
	if err != nil {
		return nil, err
	}

	pool, err := database.Open()
	if err != nil {
		return nil, err
	}
	httpServer := server.New(settings, pool)

	if err := database.Migrate(pool); err != nil {
		pool.Close()
		return nil, err
	}

	if err := deleteStaleSessionsAndLinks(pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Components{
		Server:     httpServer,
		Pool:       pool,
		ListenAddr: config.ListenAddr(),
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

	sessionRepository := repository.NewSessionRepository(pool)

	sessionRepository.DeleteAdminSessions(ctx)
	sessionRepository.DeleteExpired(ctx, sessions.IdleLimit, sessions.AbsoluteLimit)
	repository.NewOneTimeLinkRepository(pool).DeleteUsedAndExpired(ctx)

	return nil
}
