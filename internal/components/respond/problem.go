package respond

import (
	"strings"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/gofiber/fiber/v3"
)

const NotFoundMessage = "Not Found"

type problemBody struct {
	Message string `json:"message"`
}

func Problem(ctx fiber.Ctx, status int, message string) error {
	if PageRequested(ctx) {
		return pages.RenderError(ctx, status, message)
	}

	return ctx.Status(status).JSON(problemBody{Message: message})
}

func NotFound(ctx fiber.Ctx) error {
	return Problem(ctx, fiber.StatusNotFound, NotFoundMessage)
}

func PageRequested(ctx fiber.Ctx) bool {
	return pages.IsPagePath(ctx.Path()) && asksForPage(ctx)
}

func asksForPage(ctx fiber.Ctx) bool {
	return ctx.Get("HX-Request") == "true" || strings.Contains(ctx.Get(fiber.HeaderAccept), "text/html")
}
