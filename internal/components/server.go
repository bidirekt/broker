package components

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// contracts of a few thousand resources do not fit fiber's 4 MiB default
const publishBodyLimit = 8 * 1024 * 1024

const internalErrorMessage = "internal error"

type errorResponseBody struct {
	Message string `json:"message"`
}

func respondError(ctx fiber.Ctx, err error) error {
	log.Printf("request %s %s failed: %v", ctx.Method(), ctx.Path(), err)

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return ctx.Status(fiberErr.Code).JSON(errorResponseBody{Message: fiberErr.Message})
	}

	return ctx.Status(fiber.StatusInternalServerError).JSON(errorResponseBody{Message: internalErrorMessage})
}

func createHttpServer() *fiber.App {
	server := fiber.New(fiber.Config{
		BodyLimit:    publishBodyLimit,
		ErrorHandler: respondError,
	})

	server.Use(recover.New(recover.Config{EnableStackTrace: true}))

	return server
}
