package components

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

const (
	signInFailureWindow          = 15 * time.Minute
	signInFailuresPerIP          = 50
	signInFailuresPerAccount     = 10
	tooManySignInAttemptsMessage = "too many attempts, try again later"
	signInIdentifierLocalsName   = "signInIdentifier"
)

type SignInLimiter struct {
	perIP      fiber.Handler
	perAccount fiber.Handler
}

func NewSignInLimiter() *SignInLimiter {
	return &SignInLimiter{
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
			return fiber.NewError(fiber.StatusTooManyRequests, tooManySignInAttemptsMessage)
		},
	})
}

func (this *SignInLimiter) PerIP(ctx fiber.Ctx) error {
	return this.perIP(ctx)
}

func (this *SignInLimiter) PerAccount(identifierFrom func(fiber.Ctx) string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctx.Locals(signInIdentifierLocalsName, strings.ToLower(identifierFrom(ctx)))
		return this.perAccount(ctx)
	}
}
