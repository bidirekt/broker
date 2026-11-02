package sign_in

import (
	"encoding/json"
	"mime"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/bearer"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/gofiber/fiber/v3"
)

type SignInApiHandler struct {
	useCase  *SignInUseCase
	settings config.Settings
}

func NewSignInApiHandler(useCase *SignInUseCase, settings config.Settings) *SignInApiHandler {
	return &SignInApiHandler{useCase: useCase, settings: settings}
}

func (this *SignInApiHandler) RequireJsonFromPublicOrigin(ctx fiber.Ctx) error {
	if origin := ctx.Get(fiber.HeaderOrigin); origin != "" && origin != this.settings.PublicURL {
		return ctx.Status(fiber.StatusForbidden).JSON(SignInResponseBody{
			Message: ForbiddenMessage,
		})
	}

	if mediaType, _, err := mime.ParseMediaType(ctx.Get(fiber.HeaderContentType)); err != nil || mediaType != fiber.MIMEApplicationJSON {
		return this.respondInvalidInput(ctx)
	}

	return ctx.Next()
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

	token, expiresAt, err := bearer.Issue(this.settings, output.UserID.Int64, time.Now())
	if err != nil {
		return err
	}

	return ctx.Status(fiber.StatusOK).JSON(SignedInResponseBody{
		Message:   SignedInMessage,
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

func (this *SignInApiHandler) Identifier(ctx fiber.Ctx) string {
	requestBody := SignInRequestBody{}
	if err := json.Unmarshal(ctx.Body(), &requestBody); err != nil {
		return ""
	}

	return requestBody.Email
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
