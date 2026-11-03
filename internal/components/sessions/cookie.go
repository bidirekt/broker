package sessions

import (
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

const (
	secureSessionCookieName = "__Host-bidirekt_session"
	plainSessionCookieName  = "bidirekt_session"
)

func (this *Keeper) Start(ctx fiber.Ctx, userID int64, verifiedPasswordHash string) (started bool, err error) {
	cookieValue, err := tokens.Random()
	if err != nil {
		return false, err
	}
	csrfToken, err := tokens.Random()
	if err != nil {
		return false, err
	}

	session := &model.Session{IDHash: tokens.Hash(cookieValue), UserID: userID, CSRFToken: csrfToken}
	if !this.sessions.Create(ctx.Context(), session, verifiedPasswordHash) {
		return false, nil
	}

	ctx.Cookie(this.cookie(cookieValue, int(AbsoluteLimit.Seconds())))

	return true, nil
}

func (this *Keeper) ExpireCookie(ctx fiber.Ctx) {
	cookie := this.cookie("", 0)
	cookie.Expires = time.Unix(0, 0)
	ctx.Cookie(cookie)
}

func (this *Keeper) cookie(value string, maxAge int) *fiber.Cookie {
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

func (this *Keeper) cookieName() string {
	if this.isSecure() {
		return secureSessionCookieName
	}

	return plainSessionCookieName
}

func (this *Keeper) isSecure() bool {
	return strings.HasPrefix(this.settings.PublicURL, "https://")
}
