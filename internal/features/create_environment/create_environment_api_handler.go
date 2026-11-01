package create_environment

import (
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
)

type CreateEnvironmentApiHandler struct {
	useCase *CreateEnvironmentUseCase
}

func NewCreateEnvironmentApiHandler(useCase *CreateEnvironmentUseCase) *CreateEnvironmentApiHandler {
	return &CreateEnvironmentApiHandler{useCase: useCase}
}

func (this *CreateEnvironmentApiHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &CreateEnvironmentRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if validations.UnpaddedValue(requestBody.Environment) != nil {
		return this.respondInvalidInput(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), CreateEnvironmentInput{
		Environment: requestBody.Environment,
	})

	switch output.Status {
	case EnvironmentAlreadyExists:
		return this.respondAlreadyExists(ctx)
	}

	return ctx.Status(fiber.StatusOK).JSON(CreateEnvironmentResponseBody{
		Message: EnvironmentCreatedMessage,
	})
}

func (this *CreateEnvironmentApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(CreateEnvironmentResponseBody{
		Message: EnvironmentInvalidInputMessage,
	})
}

func (this *CreateEnvironmentApiHandler) respondAlreadyExists(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(CreateEnvironmentResponseBody{
		Message: EnvironmentAlreadyExistsMessage,
	})
}
