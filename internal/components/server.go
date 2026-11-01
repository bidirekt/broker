package components

import (
	"errors"
	"log"
	"strings"

	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

// contracts of a few thousand resources do not fit fiber's 4 MiB default
const publishBodyLimit = 8 * 1024 * 1024

const internalErrorMessage = "internal error"

const somethingWentWrongMessage = "something went wrong"

const pageContentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'"

type errorResponseBody struct {
	Message string `json:"message"`
}

type errorPage struct {
	Status  int
	Message string
}

func respondError(ctx fiber.Ctx, err error) error {
	log.Printf("request %s %s failed: %v", ctx.Method(), ctx.Path(), err)

	status, message := fiber.StatusInternalServerError, internalErrorMessage
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status, message = fiberErr.Code, fiberErr.Message
	}

	ctx.Response().Header.Del(fiber.HeaderLocation)
	ctx.Response().Header.Del("HX-Redirect")

	if !isPagePath(ctx) || !asksForPage(ctx) {
		return ctx.Status(status).JSON(errorResponseBody{Message: message})
	}

	if fiberErr == nil {
		message = somethingWentWrongMessage
	}
	return Render(ctx, status, templateFiles, "templates/error.html", errorPage{Status: status, Message: message})
}

func isUnderPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func isPagePath(ctx fiber.Ctx) bool {
	return !isUnderPath(ctx.Path(), "/api") && !isUnderPath(ctx.Path(), "/health") && !isUnderPath(ctx.Path(), "/assets")
}

func asksForPage(ctx fiber.Ctx) bool {
	return isHtmxRequest(ctx) || strings.Contains(ctx.Get(fiber.HeaderAccept), "text/html")
}

func createHttpServer(settings Settings, pool *pgxpool.Pool) *fiber.App {
	server := fiber.New(fiber.Config{
		BodyLimit:    publishBodyLimit,
		ErrorHandler: respondError,
	})
	server.Server().SecureErrorLogMessage = true

	server.Use(recover.New(recover.Config{EnableStackTrace: true}))
	server.Use(helmet.New(helmet.Config{
		Next:                  func(ctx fiber.Ctx) bool { return !isPagePath(ctx) },
		ContentSecurityPolicy: pageContentSecurityPolicy,
	}))
	serveAssets(server)
	server.Use(sessionDoor(&sessionKeeper{
		settings: settings,
		sessions: repository.NewSessionRepository(pool),
		users:    repository.NewUserRepository(pool),
	}))

	return server
}
