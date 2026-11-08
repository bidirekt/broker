package bearer

import (
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	apiSignInPath          = "/api/sign-in"
	bearerPrefix           = "Bearer "
	apiKeyPrefix           = "bdk_"
	lastUsedResolution     = time.Minute
	unauthenticatedMessage = "unauthenticated"
)

type Keeper struct {
	settings config.Settings
	users    *repository.UserRepository
	apiKeys  *repository.APIKeyRepository
}

func NewKeeper(settings config.Settings, users *repository.UserRepository, apiKeys *repository.APIKeyRepository) *Keeper {
	return &Keeper{settings: settings, users: users, apiKeys: apiKeys}
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
	credential, found := strings.CutPrefix(ctx.Get(fiber.HeaderAuthorization), bearerPrefix)
	if !found {
		return access.Principal{}, false
	}

	if strings.HasPrefix(credential, apiKeyPrefix) {
		return this.apiKeyToPrincipal(ctx, credential)
	}

	return this.tokenToPrincipal(ctx, credential)
}

func (this *Keeper) apiKeyToPrincipal(ctx fiber.Ctx, key string) (access.Principal, bool) {
	apiKey, owner, found := this.apiKeys.FindByHash(ctx.Context(), tokens.Hash(key))
	if !found || apiKey.ExpiresAt.Valid && !apiKey.ExpiresAt.Time.After(time.Now()) {
		return access.Principal{}, false
	}

	principal, ok := access.PrincipalCappedBy(owner, apiKey.Permission)
	if !ok {
		return access.Principal{}, false
	}

	if !apiKey.LastUsedAt.Valid || time.Since(apiKey.LastUsedAt.Time) > lastUsedResolution {
		this.apiKeys.MarkUsed(ctx.Context(), apiKey.ID)
	}

	return principal, true
}

func (this *Keeper) tokenToPrincipal(ctx fiber.Ctx, token string) (access.Principal, bool) {
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
