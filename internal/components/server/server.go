package server

import (
	"errors"
	"log"
	"strings"

	"github.com/bidirekt/broker/internal/components/bearer"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/input"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/sessions"
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

func respondError(ctx fiber.Ctx, err error) error {
	log.Printf("request %s %s failed: %v", ctx.Method(), ctx.Path(), err)

	status, message := fiber.StatusInternalServerError, internalErrorMessage
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status, message = fiberErr.Code, fiberErr.Message
	}

	ctx.Response().Header.Del(fiber.HeaderLocation)
	ctx.Response().Header.Del("HX-Redirect")

	if !pages.IsPagePath(ctx.Path()) || !asksForPage(ctx) {
		return ctx.Status(status).JSON(errorResponseBody{Message: message})
	}

	if fiberErr == nil {
		message = somethingWentWrongMessage
	}
	return pages.RenderError(ctx, status, message)
}

func asksForPage(ctx fiber.Ctx) bool {
	return ctx.Get("HX-Request") == "true" || strings.Contains(ctx.Get(fiber.HeaderAccept), "text/html")
}

func New(settings config.Settings, pool *pgxpool.Pool) *fiber.App {
	appConfig := fiber.Config{
		BodyLimit:     publishBodyLimit,
		ErrorHandler:  respondError,
		CaseSensitive: true,
		StrictRouting: true,
	}
	if len(settings.TrustedProxies) > 0 {
		appConfig.TrustProxy = true
		appConfig.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: settings.TrustedProxies}
		appConfig.ProxyHeader = fiber.HeaderXForwardedFor
		appConfig.EnableIPValidation = true
	}
	server := fiber.New(appConfig)
	server.Server().SecureErrorLogMessage = true

	server.Use(recover.New(recover.Config{EnableStackTrace: true}))
	server.Use(helmet.New(helmet.Config{
		Next:                  func(ctx fiber.Ctx) bool { return !pages.IsPagePath(ctx.Path()) },
		ContentSecurityPolicy: pageContentSecurityPolicy,
	}))
	server.Use(input.Door())
	serveAssets(server)
	users := repository.NewUserRepository(pool)
	server.Use(sessions.Door(settings, repository.NewSessionRepository(pool), users))
	server.Use(bearer.Door(settings, users))

	return server
}
