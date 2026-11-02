package sessions

import (
	"crypto/subtle"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/config"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/guregu/null"
)

const (
	IdleLimit     = 8 * time.Hour
	AbsoluteLimit = 7 * 24 * time.Hour

	secureSessionCookieName = "__Host-bidirekt_session"
	plainSessionCookieName  = "bidirekt_session"

	signInPath           = "/sign-in"
	csrfTokenHeader      = "X-CSRF-Token"
	forbiddenPageMessage = "forbidden"
)

var pathsOutsideSessionDoor = []string{"/api", "/health", "/assets", signInPath, "/set-password"}

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
	for _, prefix := range pathsOutsideSessionDoor {
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

func (this *keeper) carriesCSRFProof(ctx fiber.Ctx, session access.Session) bool {
	if !this.isSameOrigin(ctx) {
		return false
	}

	token := ctx.Get(csrfTokenHeader)

	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) == 1
}

func (this *keeper) isSameOrigin(ctx fiber.Ctx) bool {
	return ctx.Get(fiber.HeaderOrigin) == this.settings.PublicURL
}

func RequireSameOrigin(ctx fiber.Ctx) error {
	if fiber.Locals[*keeper](ctx, keeperKey{}).isSameOrigin(ctx) {
		return nil
	}

	return fiber.NewError(fiber.StatusForbidden, forbiddenPageMessage)
}

func Start(ctx fiber.Ctx, userID null.Int, verifiedPasswordHash string) (started bool, err error) {
	keeper := fiber.Locals[*keeper](ctx, keeperKey{})

	cookieValue, err := tokens.Random()
	if err != nil {
		return false, err
	}
	csrfToken, err := tokens.Random()
	if err != nil {
		return false, err
	}

	session := &model.Session{IDHash: tokens.Hash(cookieValue), UserID: userID, CSRFToken: csrfToken}
	if !keeper.sessions.Create(ctx.Context(), session, verifiedPasswordHash) {
		return false, nil
	}

	ctx.Cookie(keeper.cookie(cookieValue, int(AbsoluteLimit.Seconds())))

	return true, nil
}

func ExpireCookie(ctx fiber.Ctx) {
	keeper := fiber.Locals[*keeper](ctx, keeperKey{})
	cookie := keeper.cookie("", 0)
	cookie.Expires = time.Unix(0, 0)
	ctx.Cookie(cookie)
}

func (this *keeper) cookie(value string, maxAge int) *fiber.Cookie {
	return &fiber.Cookie{
		Name:     this.cookieName(),
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
		Secure:   this.isSecure(),
	}
}

func (this *keeper) cookieName() string {
	if this.isSecure() {
		return secureSessionCookieName
	}

	return plainSessionCookieName
}

func (this *keeper) isSecure() bool {
	return strings.HasPrefix(this.settings.PublicURL, "https://")
}

func isUnderPath(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
