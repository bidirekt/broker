package components

import (
	"strings"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	apiPath                = "/api"
	apiSignInPath          = "/api/sign-in"
	bearerPrefix           = "Bearer "
	unauthenticatedMessage = "unauthenticated"
)

type bearerKeeper struct {
	settings Settings
	users    *repository.UserRepository
}

func bearerDoor(keeper *bearerKeeper) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if !isUnderPath(ctx.Path(), apiPath) || ctx.Path() == apiSignInPath {
			return ctx.Next()
		}

		principal, ok := keeper.toPrincipal(ctx)
		if !ok {
			ctx.Set(fiber.HeaderWWWAuthenticate, "Bearer")
			return fiber.NewError(fiber.StatusUnauthorized, unauthenticatedMessage)
		}

		ctx.Locals(principalKey{}, principal)

		return ctx.Next()
	}
}

func (this *bearerKeeper) toPrincipal(ctx fiber.Ctx) (Principal, bool) {
	token, found := strings.CutPrefix(ctx.Get(fiber.HeaderAuthorization), bearerPrefix)
	if !found {
		return Principal{}, false
	}

	userID, valid := UserIDFromToken(this.settings, token)
	if !valid {
		return Principal{}, false
	}

	user, found := this.users.FindByID(ctx.Context(), userID)
	if !found || user.State() != model.UserActive {
		return Principal{}, false
	}

	return Principal{UserID: user.ID, Permission: user.Permission}, true
}
