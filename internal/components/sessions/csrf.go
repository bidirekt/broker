package sessions

import (
	"crypto/subtle"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/gofiber/fiber/v3"
)

const csrfTokenHeader = "X-CSRF-Token"

func (this *keeper) carriesCSRFProof(ctx fiber.Ctx, session access.Session) bool {
	if !this.isSameOrigin(ctx) {
		return false
	}

	token := ctx.Get(csrfTokenHeader)

	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) == 1
}

func (this *keeper) isSameOrigin(ctx fiber.Ctx) bool {
	return ctx.Get(fiber.HeaderOrigin) == this.settings.PublicURL
}

func IsSameOrigin(ctx fiber.Ctx) bool {
	return fiber.Locals[*keeper](ctx, keeperKey{}).isSameOrigin(ctx)
}
