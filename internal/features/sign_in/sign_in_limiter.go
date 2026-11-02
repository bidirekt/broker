package sign_in

import (
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

const (
	signInFailureWindow      = 15 * time.Minute
	signInFailuresPerIP      = 50
	signInFailuresPerAccount = 10
	signInEmailLocalsName    = "signInEmail"
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
			email, _ := ctx.Locals(signInEmailLocalsName).(string)
			return ctx.IP() + " " + email
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
		LimitReached: func(ctx fiber.Ctx) error {
			return respond.Problem(ctx, fiber.StatusTooManyRequests, TooManySignInAttemptsMessage)
		},
	})
}

func (this *signInLimiter) PerIP(ctx fiber.Ctx) error {
	return this.perIP(ctx)
}

func (this *signInLimiter) PerAccount(emailFrom func(fiber.Ctx) string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctx.Locals(signInEmailLocalsName, strings.ToLower(emailFrom(ctx)))
		return this.perAccount(ctx)
	}
}
