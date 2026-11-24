package server

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

//go:embed assets
var assetFiles embed.FS

func serveAssets(server *fiber.App, sections []breakreport.ProtocolSection) {
	for _, section := range sections {
		stylesheet := section.ReportKey() + ".css"
		server.Get("/assets/protocols/"+stylesheet, static.New(stylesheet, static.Config{FS: section.Files()}))
	}
	server.Use("/assets", static.New("assets", static.Config{FS: assetFiles}))
}
