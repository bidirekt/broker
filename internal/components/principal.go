package components

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
	IsAdmin    bool
}

type CurrentSession struct {
	IDHash    []byte
	CSRFToken string
}

type principalKey struct{}

type currentSessionKey struct{}

func PrincipalFrom(ctx fiber.Ctx) (Principal, bool) {
	principal, ok := ctx.Locals(principalKey{}).(Principal)
	return principal, ok
}

func SessionFrom(ctx fiber.Ctx) (CurrentSession, bool) {
	session, ok := ctx.Locals(currentSessionKey{}).(CurrentSession)
	return session, ok
}

func RequireAdmin(ctx fiber.Ctx) error {
	if principal, ok := PrincipalFrom(ctx); ok && principal.IsAdmin {
		return nil
	}

	return fiber.NewError(fiber.StatusForbidden, noAccessMessage)
}

func RequireUser(ctx fiber.Ctx) error {
	if principal, ok := PrincipalFrom(ctx); ok && !principal.IsAdmin {
		return nil
	}

	return fiber.NewError(fiber.StatusForbidden, noAccessMessage)
}

func RequirePermission(ctx fiber.Ctx, permission model.Permission) error {
	principal, ok := PrincipalFrom(ctx)
	if ok && (principal.Permission == permission || principal.Permission == model.PermissionWrite) {
		return nil
	}

	return fiber.NewError(fiber.StatusForbidden, forbiddenApiMessage)
}
