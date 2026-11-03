package server

import (
	"embed"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

//go:embed assets
var assetFiles embed.FS

func serveAssets(server *fiber.App) {
	server.Use("/assets", static.New("assets", static.Config{FS: assetFiles}))
}
