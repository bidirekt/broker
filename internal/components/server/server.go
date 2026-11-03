package server

import (
	"errors"
	"log"

	"github.com/bidirekt/broker/internal/components/bearer"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/input"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
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

func respondError(ctx fiber.Ctx, err error) error {
	log.Printf("request %s %s failed: %v", ctx.Method(), ctx.Path(), err)

	ctx.Response().Header.Del(fiber.HeaderLocation)
	ctx.Response().Header.Del("HX-Redirect")

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return respond.Problem(ctx, fiberErr.Code, fiberErr.Message)
	}

	message := internalErrorMessage
	if respond.PageRequested(ctx) {
		message = somethingWentWrongMessage
	}
	return respond.Problem(ctx, fiber.StatusInternalServerError, message)
}

func New(settings config.Settings, pool *pgxpool.Pool, sessionKeeper *sessions.Keeper) *fiber.App {
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
	server.Use(sessionKeeper.Door)
	server.Use(bearer.Door(settings, repository.NewUserRepository(pool)))

	return server
}
