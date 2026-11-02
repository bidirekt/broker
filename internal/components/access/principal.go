package access

import (
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

const (
	noAccessMessage     = "you don't have access"
	forbiddenApiMessage = "forbidden"
)

type Principal struct {
	UserID     int64
	Permission model.Permission
}

type Session struct {
	IDHash    []byte
	CSRFToken string
}

type principalKey struct{}

type sessionKey struct{}

func PrincipalOf(user *model.User) (Principal, bool) {
	if user.State() != model.UserActive {
		return Principal{}, false
	}

	return Principal{UserID: user.ID, Permission: user.Permission}, true
}

func (this Principal) ManagesUsers() bool {
	return this.Permission == model.PermissionAdmin
}

func GrantPrincipal(ctx fiber.Ctx, principal Principal) {
	ctx.Locals(principalKey{}, principal)
}

func GrantSession(ctx fiber.Ctx, session Session) {
	ctx.Locals(sessionKey{}, session)
}

func PrincipalFrom(ctx fiber.Ctx) (Principal, bool) {
	principal, ok := ctx.Locals(principalKey{}).(Principal)
	return principal, ok
}

func SessionFrom(ctx fiber.Ctx) (Session, bool) {
	session, ok := ctx.Locals(sessionKey{}).(Session)
	return session, ok
}

func NoAccess() error {
	return fiber.NewError(fiber.StatusForbidden, noAccessMessage)
}

func RequireAdmin(ctx fiber.Ctx) error {
	if principal, ok := PrincipalFrom(ctx); ok && principal.ManagesUsers() {
		return nil
	}

	return NoAccess()
}

func RequirePermission(ctx fiber.Ctx, permission model.Permission) error {
	principal, ok := PrincipalFrom(ctx)
	if ok && principal.Permission.Includes(permission) {
		return nil
	}

	return fiber.NewError(fiber.StatusForbidden, forbiddenApiMessage)
}
