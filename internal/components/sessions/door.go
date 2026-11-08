package sessions

import (
	"net/url"
	"sync/atomic"
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
	IdleLimit     = 8 * time.Hour
	AbsoluteLimit = 7 * 24 * time.Hour

	signInPath = "/sign-in"
	setupPath  = "/setup"
)

var pagesOutsideSessionDoor = []string{signInPath, "/set-password", setupPath}

type Keeper struct {
	settings config.Settings
	sessions *repository.SessionRepository
	users    *repository.UserRepository
	// no row becomes or stops being the admin through the broker, so once true it stays true
	adminExists atomic.Bool
}

func NewKeeper(settings config.Settings, sessions *repository.SessionRepository, users *repository.UserRepository) *Keeper {
	return &Keeper{settings: settings, sessions: sessions, users: users}
}

func (this *Keeper) Door(ctx fiber.Ctx) error {
	if !pages.IsPagePath(ctx.Path()) {
		return ctx.Next()
	}

	if !pages.IsUnderPath(ctx.Path(), setupPath) && !this.isSetUp(ctx) {
		return pages.Redirect(ctx, setupPath)
	}

	if isPageOutsideSessionDoor(ctx.Path()) {
		return ctx.Next()
	}

	principal, session, ok := this.admitSession(ctx)
	if !ok {
		return pages.Redirect(ctx, signInPathFrom(ctx))
	}

	access.GrantPrincipal(ctx, principal)
	access.GrantSession(ctx, session)

	if !isSafeMethod(ctx.Method()) && !this.carriesCSRFProof(ctx, session) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.ForbiddenMessage)
	}

	return ctx.Next()
}

func (this *Keeper) isSetUp(ctx fiber.Ctx) bool {
	if this.adminExists.Load() {
		return true
	}

	if !this.users.AdminExists(ctx.Context()) {
		return false
	}

	this.adminExists.Store(true)

	return true
}

func isPageOutsideSessionDoor(path string) bool {
	for _, prefix := range pagesOutsideSessionDoor {
		if pages.IsUnderPath(path, prefix) {
			return true
		}
	}

	return false
}

func signInPathFrom(ctx fiber.Ctx) string {
	if !isSafeMethod(ctx.Method()) {
		return signInPath
	}

	return signInPath + "?next=" + url.QueryEscape(ctx.OriginalURL())
}

func isSafeMethod(method string) bool {
	return method == fiber.MethodGet || method == fiber.MethodHead
}

func (this *Keeper) admitSession(ctx fiber.Ctx) (access.Principal, access.Session, bool) {
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

	user, found := this.users.FindByID(ctx.Context(), session.UserID)
	if !found {
		return access.Principal{}, access.Session{}, false
	}

	principal, active := access.PrincipalOf(user)
	if !active {
		return access.Principal{}, access.Session{}, false
	}

	admitted := access.Session{IDHash: idHash, CSRFToken: session.CSRFToken}
	if ctx.Method() == fiber.MethodGet && respond.PageRequested(ctx) {
		admitted.Messages = this.sessions.MarkSeenTakingMessages(ctx.Context(), idHash)
	} else {
		this.sessions.MarkSeen(ctx.Context(), idHash)
	}

	return principal, admitted, true
}
