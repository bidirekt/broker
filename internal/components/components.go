package components

import (
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/database"
	"github.com/bidirekt/broker/internal/components/server"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
