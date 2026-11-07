package access

import (
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

const (
	NoAccessMessage  = "you don't have access"
	ForbiddenMessage = "forbidden"
)

type Principal struct {
	UserID     int64
	Email      string
	Permission model.Permission
}

type Session struct {
	IDHash    []byte
	CSRFToken string
	Messages  []model.SessionMessage
}

type principalKey struct{}

type sessionKey struct{}

func PrincipalOf(user *model.User) (Principal, bool) {
	if user.State() != model.UserActive {
		return Principal{}, false
	}

	return Principal{UserID: user.ID, Email: user.Email, Permission: user.Permission}, true
}

func (this Principal) ManagesUsers() bool {
	return this.Permission == model.PermissionAdmin
}

func (this Principal) CanWrite() bool {
	return this.Permission.Includes(model.PermissionWrite)
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

func IsAdmin(ctx fiber.Ctx) bool {
	principal, ok := PrincipalFrom(ctx)
	return ok && principal.ManagesUsers()
}

func Allows(ctx fiber.Ctx, permission model.Permission) bool {
	principal, ok := PrincipalFrom(ctx)
	return ok && principal.Permission.Includes(permission)
}
