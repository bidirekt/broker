package components

import (
	"github.com/bidirekt/broker/internal/components/bearer"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/database"
	"github.com/bidirekt/broker/internal/components/server"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Components struct {
	Server        *fiber.App
	Pool          *pgxpool.Pool
	ListenAddr    string
	Settings      config.Settings
	SessionKeeper *sessions.Keeper
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
	users := repository.NewUserRepository(pool)
	sessionKeeper := sessions.NewKeeper(settings, repository.NewSessionRepository(pool), users)
	bearerKeeper := bearer.NewKeeper(settings, users, repository.NewAPIKeyRepository(pool))
	httpServer := server.New(settings, sessionKeeper, bearerKeeper)

	if err := database.Migrate(pool); err != nil {
		pool.Close()
		return nil, err
	}

	if err := deleteExpiredCredentials(pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Components{
		Server:        httpServer,
		Pool:          pool,
		ListenAddr:    config.ListenAddr(),
		Settings:      settings,
		SessionKeeper: sessionKeeper,
	}, nil
}
