package list_environments

import (
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

type ListEnvironmentsHandler struct {
	environmentRepository *repository.EnvironmentRepository
}

func NewListEnvironmentsHandler(repo *repository.EnvironmentRepository) *ListEnvironmentsHandler {
	return &ListEnvironmentsHandler{environmentRepository: repo}
}

func (this *ListEnvironmentsHandler) Handle(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(ListEnvironmentsResponseBody{
		Message:      EnvironmentsListed,
		Environments: this.environmentRepository.ListNames(ctx.Context()),
	})
}
