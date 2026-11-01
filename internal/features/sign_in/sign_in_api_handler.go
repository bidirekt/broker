package sign_in

import (
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

type SignInApiHandler struct {
	useCase  *SignInUseCase
	settings components.Settings
}

func NewSignInApiHandler(useCase *SignInUseCase, settings components.Settings) *SignInApiHandler {
	return &SignInApiHandler{useCase: useCase, settings: settings}
}

func (this *SignInApiHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &SignInRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.Email == "" || requestBody.Password == "" {
		return this.respondInvalidInput(ctx)
	}

	if !strings.Contains(requestBody.Email, "@") {
		return this.respondInvalidCredentials(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), SignInInput{
		Identifier: requestBody.Email,
		Password:   requestBody.Password,
	})

	if output.Status == InvalidCredentials || !output.UserID.Valid {
		return this.respondInvalidCredentials(ctx)
	}

	token, expiresAt, err := components.IssueToken(this.settings, output.UserID.Int64, time.Now())
	if err != nil {
		return err
	}

	return ctx.Status(fiber.StatusOK).JSON(SignedInResponseBody{
		Message:   SignedInMessage,
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

func (this *SignInApiHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(SignInResponseBody{
		Message: SignInInvalidInputMessage,
	})
}

func (this *SignInApiHandler) respondInvalidCredentials(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusUnauthorized).JSON(SignInResponseBody{
		Message: ApiInvalidCredentialsMessage,
	})
}
