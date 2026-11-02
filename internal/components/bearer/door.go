package bearer

import (
	"strings"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	apiPath                = "/api"
	apiSignInPath          = "/api/sign-in"
	bearerPrefix           = "Bearer "
	unauthenticatedMessage = "unauthenticated"
)

type keeper struct {
	settings config.Settings
	users    *repository.UserRepository
}

func Door(settings config.Settings, users *repository.UserRepository) fiber.Handler {
	keeper := &keeper{settings: settings, users: users}

	return func(ctx fiber.Ctx) error {
		if !isUnderPath(ctx.Path(), apiPath) || ctx.Path() == apiSignInPath {
			return ctx.Next()
		}

		principal, ok := keeper.toPrincipal(ctx)
		if !ok {
			ctx.Set(fiber.HeaderWWWAuthenticate, "Bearer")
			return respond.Problem(ctx, fiber.StatusUnauthorized, unauthenticatedMessage)
		}

		access.GrantPrincipal(ctx, principal)

		return ctx.Next()
	}
}

func (this *keeper) toPrincipal(ctx fiber.Ctx) (access.Principal, bool) {
	token, found := strings.CutPrefix(ctx.Get(fiber.HeaderAuthorization), bearerPrefix)
	if !found {
		return access.Principal{}, false
	}

	userID, valid := UserIDFrom(this.settings, token)
	if !valid {
		return access.Principal{}, false
	}

	user, found := this.users.FindByID(ctx.Context(), userID)
	if !found {
		return access.Principal{}, false
	}

	return access.PrincipalOf(user)
}

func isUnderPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
