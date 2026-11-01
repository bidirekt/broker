package components

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Components struct {
	Server     *fiber.App
	Pool       *pgxpool.Pool
	ListenAddr string
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
