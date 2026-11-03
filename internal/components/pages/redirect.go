package pages

import "github.com/gofiber/fiber/v3"

func Redirect(ctx fiber.Ctx, path string) error {
	if isHtmxRequest(ctx) {
		ctx.Set("HX-Redirect", path)
		return ctx.SendStatus(fiber.StatusOK)
	}

	return ctx.Redirect().Status(fiber.StatusSeeOther).To(path)
}
