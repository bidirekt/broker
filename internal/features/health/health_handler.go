package health

import (
	"github.com/bidirekt/broker/internal/components/versions"
	"github.com/gofiber/fiber/v3"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (this *HealthHandler) Handle(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(HealthResponseBody{
		Status:        HealthOK,
		BrokerVersion: versions.Broker(),
		APIVersion:    apiVersion,
	})
}
