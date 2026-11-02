package sessions

import (
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	IdleLimit     = 8 * time.Hour
	AbsoluteLimit = 7 * 24 * time.Hour

	signInPath = "/sign-in"
)

var pagesOutsideSessionDoor = []string{signInPath, "/set-password"}

type keeper struct {
	settings config.Settings
	sessions *repository.SessionRepository
	users    *repository.UserRepository
}

type keeperKey struct{}

func Door(settings config.Settings, sessions *repository.SessionRepository, users *repository.UserRepository) fiber.Handler {
	keeper := &keeper{settings: settings, sessions: sessions, users: users}

	return func(ctx fiber.Ctx) error {
		ctx.Locals(keeperKey{}, keeper)

		if isOutsideSessionDoor(ctx.Path()) {
			return ctx.Next()
		}

		principal, session, ok := keeper.toPrincipal(ctx)
		if !ok {
			return pages.Redirect(ctx, signInPath)
		}

		access.GrantPrincipal(ctx, principal)
		access.GrantSession(ctx, session)

		if !isSafeMethod(ctx.Method()) && !keeper.carriesCSRFProof(ctx, session) {
			return fiber.NewError(fiber.StatusForbidden, forbiddenPageMessage)
		}

		return ctx.Next()
	}
}

func isOutsideSessionDoor(path string) bool {
	if !pages.IsPagePath(path) {
		return true
	}

	for _, prefix := range pagesOutsideSessionDoor {
		if isUnderPath(path, prefix) {
			return true
		}
	}

	return false
}

func isSafeMethod(method string) bool {
	return method == fiber.MethodGet || method == fiber.MethodHead
}

func (this *keeper) toPrincipal(ctx fiber.Ctx) (access.Principal, access.Session, bool) {
	cookieValue := ctx.Cookies(this.cookieName())
	if cookieValue == "" {
		return access.Principal{}, access.Session{}, false
	}

	idHash := tokens.Hash(cookieValue)
	session, found := this.sessions.FindByIDHash(ctx.Context(), idHash)
	if !found {
		return access.Principal{}, access.Session{}, false
	}

	now := time.Now()
	if now.Sub(session.LastSeenAt) >= IdleLimit || now.Sub(session.CreatedAt) >= AbsoluteLimit {
		this.sessions.Delete(ctx.Context(), idHash)
		return access.Principal{}, access.Session{}, false
	}

	principal := access.Principal{IsAdmin: true}
	if session.UserID.Valid {
		user, found := this.users.FindByID(ctx.Context(), session.UserID.Int64)
		if !found || user.State() != model.UserActive {
			return access.Principal{}, access.Session{}, false
		}
		principal = access.Principal{UserID: user.ID, Permission: user.Permission}
	}

	this.sessions.MarkSeen(ctx.Context(), idHash)

	return principal, access.Session{IDHash: idHash, CSRFToken: session.CSRFToken}, true
}

func isUnderPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
