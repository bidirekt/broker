package components

import (
	"crypto/subtle"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/guregu/null"
)

const (
	sessionIdleLimit     = 8 * time.Hour
	sessionAbsoluteLimit = 7 * 24 * time.Hour

	secureSessionCookieName = "__Host-bidirekt_session"
	plainSessionCookieName  = "bidirekt_session"

	signInPath           = "/sign-in"
	csrfTokenHeader      = "X-CSRF-Token"
	forbiddenPageMessage = "forbidden"
)

var pathsOutsideSessionDoor = []string{"/api", "/health", "/assets", signInPath, "/set-password"}

type sessionKeeper struct {
	settings Settings
	sessions *repository.SessionRepository
	users    *repository.UserRepository
}

type sessionKeeperKey struct{}

func sessionDoor(keeper *sessionKeeper) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctx.Locals(sessionKeeperKey{}, keeper)

		if isOutsideSessionDoor(ctx.Path()) {
			return ctx.Next()
		}

		principal, session, ok := keeper.toPrincipal(ctx)
		if !ok {
			return Redirect(ctx, signInPath)
		}

		ctx.Locals(principalKey{}, principal)
		ctx.Locals(currentSessionKey{}, session)

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

func (this *sessionKeeper) toPrincipal(ctx fiber.Ctx) (Principal, CurrentSession, bool) {
	cookieValue := ctx.Cookies(this.cookieName())
	if cookieValue == "" {
		return Principal{}, CurrentSession{}, false
	}

	idHash := HashToken(cookieValue)
	session, found := this.sessions.FindByIDHash(ctx.Context(), idHash)
	if !found {
		return Principal{}, CurrentSession{}, false
	}

	now := time.Now()
	if now.Sub(session.LastSeenAt) >= sessionIdleLimit || now.Sub(session.CreatedAt) >= sessionAbsoluteLimit {
		this.sessions.Delete(ctx.Context(), idHash)
		return Principal{}, CurrentSession{}, false
	}

	principal := Principal{IsAdmin: true}
	if session.UserID.Valid {
		user, found := this.users.FindByID(ctx.Context(), session.UserID.Int64)
		if !found || user.State() != model.UserActive {
			return Principal{}, CurrentSession{}, false
		}
		principal = Principal{UserID: user.ID, Permission: user.Permission}
	}

	this.sessions.MarkSeen(ctx.Context(), idHash)

	return principal, CurrentSession{IDHash: idHash, CSRFToken: session.CSRFToken}, true
}

func (this *sessionKeeper) carriesCSRFProof(ctx fiber.Ctx, session CurrentSession) bool {
	if !this.isSameOrigin(ctx) {
		return false
	}

	token := ctx.Get(csrfTokenHeader)

	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) == 1
}

func (this *sessionKeeper) isSameOrigin(ctx fiber.Ctx) bool {
	return ctx.Get(fiber.HeaderOrigin) == this.settings.PublicURL
}

func RequireSameOrigin(ctx fiber.Ctx) error {
	if fiber.Locals[*sessionKeeper](ctx, sessionKeeperKey{}).isSameOrigin(ctx) {
		return nil
	}

	return fiber.NewError(fiber.StatusForbidden, forbiddenPageMessage)
}

func Redirect(ctx fiber.Ctx, path string) error {
	if isHtmxRequest(ctx) {
		ctx.Set("HX-Redirect", path)
		return ctx.SendStatus(fiber.StatusOK)
	}

	return ctx.Redirect().Status(fiber.StatusSeeOther).To(path)
}

func StartSession(ctx fiber.Ctx, userID null.Int, verifiedPasswordHash string) (started bool, err error) {
	keeper := fiber.Locals[*sessionKeeper](ctx, sessionKeeperKey{})

	cookieValue, err := RandomToken()
	if err != nil {
		return false, err
	}
	csrfToken, err := RandomToken()
	if err != nil {
		return false, err
	}

	session := &model.Session{IDHash: HashToken(cookieValue), UserID: userID, CSRFToken: csrfToken}
	if !keeper.sessions.Create(ctx.Context(), session, verifiedPasswordHash) {
		return false, nil
	}

	ctx.Cookie(keeper.cookie(cookieValue, int(sessionAbsoluteLimit.Seconds())))

	return true, nil
}

func ExpireSessionCookie(ctx fiber.Ctx) {
	keeper := fiber.Locals[*sessionKeeper](ctx, sessionKeeperKey{})
	cookie := keeper.cookie("", 0)
	cookie.Expires = time.Unix(0, 0)
	ctx.Cookie(cookie)
}

func (this *sessionKeeper) cookie(value string, maxAge int) *fiber.Cookie {
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

func (this *sessionKeeper) cookieName() string {
	if this.isSecure() {
		return secureSessionCookieName
	}

	return plainSessionCookieName
}

func (this *sessionKeeper) isSecure() bool {
	return strings.HasPrefix(this.settings.PublicURL, "https://")
}
