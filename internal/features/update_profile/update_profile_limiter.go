package update_profile

import (
	"strconv"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

const (
	profileFailureWindow   = 15 * time.Minute
	profileFailuresPerUser = 10
)

func newUpdateProfileLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:                    profileFailuresPerUser,
		Expiration:             profileFailureWindow,
		LimiterMiddleware:      limiter.SlidingWindow{},
		SkipSuccessfulRequests: true,
		KeyGenerator: func(ctx fiber.Ctx) string {
			principal, _ := access.PrincipalFrom(ctx)
			return strconv.FormatInt(principal.UserID, 10)
		},
		LimitReached: func(ctx fiber.Ctx) error {
			return respond.Problem(ctx, fiber.StatusTooManyRequests, TooManyProfileAttemptsMessage)
		},
	})
}
