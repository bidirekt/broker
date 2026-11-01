package sign_in

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

const (
	signInFailureWindow        = 15 * time.Minute
	signInFailuresPerIP        = 50
	signInFailuresPerAccount   = 10
	signInIdentifierLocalsName = "signInIdentifier"
)

type signInLimiter struct {
	perIP      fiber.Handler
	perAccount fiber.Handler
}

func newSignInLimiter() *signInLimiter {
	return &signInLimiter{
		perIP: newSignInFailureLimiter(signInFailuresPerIP, func(ctx fiber.Ctx) string {
			return ctx.IP()
		}),
		perAccount: newSignInFailureLimiter(signInFailuresPerAccount, func(ctx fiber.Ctx) string {
			identifier, _ := ctx.Locals(signInIdentifierLocalsName).(string)
			return ctx.IP() + " " + identifier
		}),
	}
}

func newSignInFailureLimiter(maxFailures int, key func(fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:                    maxFailures,
		Expiration:             signInFailureWindow,
		LimiterMiddleware:      limiter.SlidingWindow{},
		SkipSuccessfulRequests: true,
		KeyGenerator:           key,
		LimitReached: func(fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, TooManySignInAttemptsMessage)
		},
	})
}

func (this *signInLimiter) PerIP(ctx fiber.Ctx) error {
	return this.perIP(ctx)
}

func (this *signInLimiter) PerAccount(identifierFrom func(fiber.Ctx) string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctx.Locals(signInIdentifierLocalsName, strings.ToLower(identifierFrom(ctx)))
		return this.perAccount(ctx)
	}
}
