package bearer

import (
	"strings"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	apiSignInPath          = "/api/sign-in"
	bearerPrefix           = "Bearer "
	unauthenticatedMessage = "unauthenticated"
)

type Keeper struct {
	settings config.Settings
	users    *repository.UserRepository
}

func NewKeeper(settings config.Settings, users *repository.UserRepository) *Keeper {
	return &Keeper{settings: settings, users: users}
}

func (this *Keeper) Door(ctx fiber.Ctx) error {
	if !pages.IsAPIPath(ctx.Path()) || ctx.Path() == apiSignInPath {
		return ctx.Next()
	}

	principal, ok := this.toPrincipal(ctx)
	if !ok {
		ctx.Set(fiber.HeaderWWWAuthenticate, "Bearer")
		return respond.Problem(ctx, fiber.StatusUnauthorized, unauthenticatedMessage)
	}

	access.GrantPrincipal(ctx, principal)

	return ctx.Next()
}

func (this *Keeper) toPrincipal(ctx fiber.Ctx) (access.Principal, bool) {
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
