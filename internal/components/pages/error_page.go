package pages

import "github.com/gofiber/fiber/v3"

type errorPage struct {
	Status  int
	Message string
}

func RenderError(ctx fiber.Ctx, status int, message string) error {
	return Render(ctx, status, templateFiles, "templates/error.html", errorPage{Status: status, Message: message})
}
