package integration_test

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	doorSessionCookieName = "bidirekt_session"
	doorSessionCookie     = "door-session-cookie"
	doorCSRFToken         = "door-csrf-token"
	doorPublicURL         = "http://localhost:8080"
	doorUnknownPage       = "/no-such-page-behind-the-door"
	doorUserPasswordHash  = "some-hash"
)

type doorRequest struct {
	method  string
	path    string
	cookie  string
	headers map[string]string
}

func (this *IntegrationSuite) sendThroughDoor(request doorRequest) (status int, header http.Header, body string) {
	req := httptest.NewRequest(request.method, request.path, nil)
	req.Header.Set("Accept", "text/html")
	if request.cookie != "" {
		req.AddCookie(&http.Cookie{Name: doorSessionCookieName, Value: request.cookie})
	}
	for name, value := range request.headers {
		req.Header.Set(name, value)
	}

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) createDoorSession(userID int64) []byte {
	idHash := sha256.Sum256([]byte(doorSessionCookie))
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO sessions (id_hash, user_id, csrf_token) VALUES ($1, $2, $3)`,
		idHash[:], userID, doorCSRFToken,
	)
	this.Require().NoError(err)
	return idHash[:]
}

func (this *IntegrationSuite) createDoorUser(active bool) (userID int64) {
	err := this.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, permission, password_hash, active) VALUES ('ana@x.com', 'write', $1, $2) RETURNING id`,
		doorUserPasswordHash, active,
	).Scan(&userID)
	this.Require().NoError(err)
	return userID
}

func (this *IntegrationSuite) TestSessionDoor_WithoutCookie_RedirectsToSignIn() {
	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSessionDoor_HtmxRequestWithoutCookie_AnswersHXRedirect() {
	status, header, _ := this.sendThroughDoor(doorRequest{
		method: "GET", path: doorUnknownPage, headers: map[string]string{"HX-Request": "true"},
	})
	this.Equal(http.StatusOK, status)
	this.Equal("/sign-in", header.Get("HX-Redirect"))
	this.Empty(header.Get("Location"))
}

func (this *IntegrationSuite) TestSessionDoor_UnknownCookie_RedirectsToSignIn() {
	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: "never-issued"})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSessionDoor_ActiveUserSession_PassesTheDoorAndBumpsLastSeen() {
	idHash := this.createDoorSession(this.createDoorUser(true))
	_, err := this.Pool.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - interval '1 hour'`)
	this.Require().NoError(err)

	status, header, body := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, `action="/sign-out"`)
	this.Contains(body, doorCSRFToken)

	session, found := repository.NewSessionRepository(this.Pool).FindByIDHash(context.Background(), idHash)
	this.Require().True(found)
	this.WithinDuration(time.Now(), session.LastSeenAt, time.Minute)
}

func (this *IntegrationSuite) TestSessionDoor_AdminSession_PassesTheDoor() {
	this.createDoorSession(this.adminID)

	status, _, body := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusNotFound, status)
	this.Contains(body, `href="/users"`)
}

func (this *IntegrationSuite) TestSessionDoor_IdleSession_RedirectsAndIsDeleted() {
	this.createDoorSession(this.adminID)
	_, err := this.Pool.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - interval '8 hours 1 minute'`)
	this.Require().NoError(err)

	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionDoor_SessionOlderThanSevenDays_Redirects() {
	this.createDoorSession(this.adminID)
	_, err := this.Pool.Exec(context.Background(), `UPDATE sessions SET created_at = now() - interval '7 days 1 minute'`)
	this.Require().NoError(err)

	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSessionDoor_DeactivatedUserSession_Redirects() {
	this.createDoorSession(this.createDoorUser(false))

	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSessionDoor_PostWithoutCSRFToken_Answers403() {
	this.createDoorSession(this.adminID)

	status, header, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL},
	})
	this.Equal(http.StatusForbidden, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
}

func (this *IntegrationSuite) TestSessionDoor_PostWithWrongCSRFToken_Answers403() {
	this.createDoorSession(this.adminID)

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL, "X-CSRF-Token": "not-the-token"},
	})
	this.Equal(http.StatusForbidden, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithForeignOrigin_Answers403() {
	this.createDoorSession(this.adminID)

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": "http://evil.example", "X-CSRF-Token": doorCSRFToken},
	})
	this.Equal(http.StatusForbidden, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithoutOrigin_Answers403() {
	this.createDoorSession(this.adminID)

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"X-CSRF-Token": doorCSRFToken},
	})
	this.Equal(http.StatusForbidden, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithCSRFHeaderAndOrigin_PassesTheDoor() {
	this.createDoorSession(this.adminID)

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL, "X-CSRF-Token": doorCSRFToken},
	})
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestSessionDoor_LeavesApiHealthAndAssetsOpen() {
	for _, path := range []string{"/api/participants", "/health", "/assets/app.css"} {
		status, header, _ := this.sendThroughDoor(doorRequest{
			method:  "GET",
			path:    path,
			headers: map[string]string{"Authorization": "Bearer " + this.writerToken},
		})
		this.Equal(http.StatusOK, status, path)
		this.Empty(header.Get("Location"), path)
	}
}

func (this *IntegrationSuite) TestSessionDoor_LeavesSignInAndSetPasswordSubPathsOpen() {
	for _, path := range []string{"/sign-in/no-such-page", "/set-password/no-such-page"} {
		status, _, _ := this.sendThroughDoor(doorRequest{method: "GET", path: path})
		this.Equal(http.StatusNotFound, status, path)
	}
}

func (this *IntegrationSuite) TestSessionDoor_AnotherCaseOfAPagePathIsNotRouted() {
	this.createDoorSession(this.adminID)

	for _, path := range []string{"/USERS", "/SIGN-IN"} {
		status, _, _ := this.sendThroughDoor(doorRequest{method: "GET", path: path, cookie: doorSessionCookie})
		this.Equal(http.StatusNotFound, status, path)
	}
}

func (this *IntegrationSuite) TestSessionDoor_SignInAndSetPasswordWithATrailingSlashAreNotRouted() {
	for _, path := range []string{"/sign-in/", "/set-password/"} {
		status, _, _ := this.sendThroughDoor(doorRequest{method: "GET", path: path})
		this.Equal(http.StatusNotFound, status, path)
	}
}

func (this *IntegrationSuite) TestSessionDoor_LookalikePrefixStaysBehindTheDoor() {
	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: "/sign-inside"})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestStartSession_SetsCookieThatPassesTheDoor() {
	userID := this.createDoorUser(true)
	this.Components.Server.Get("/sign-in/__start_session", func(ctx fiber.Ctx) error {
		started, err := sessions.Start(ctx, userID, doorUserPasswordHash)
		if err != nil {
			return err
		}
		if !started {
			return ctx.SendStatus(http.StatusUnauthorized)
		}
		return ctx.SendStatus(http.StatusNoContent)
	})

	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: "/sign-in/__start_session"})
	this.Require().Equal(http.StatusNoContent, status)

	setCookie := header.Get("Set-Cookie")
	this.Contains(setCookie, "bidirekt_session=")
	this.Contains(setCookie, "max-age=604800")
	this.Contains(setCookie, "path=/")
	this.Contains(strings.ToLower(setCookie), "httponly")
	this.Contains(strings.ToLower(setCookie), "samesite=lax")
	this.NotContains(strings.ToLower(setCookie), "secure")

	cookies, err := http.ParseSetCookie(setCookie)
	this.Require().NoError(err)
	this.Len(cookies.Value, 43)
	this.Equal(1, this.countRows("sessions"))

	status, _, _ = this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: cookies.Value})
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestExpireSessionCookie_ClearsTheCookie() {
	this.Components.Server.Get("/sign-in/__expire_session", func(ctx fiber.Ctx) error {
		sessions.ExpireCookie(ctx)
		return ctx.SendStatus(http.StatusNoContent)
	})

	_, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: "/sign-in/__expire_session"})

	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Equal(doorSessionCookieName, cookie.Name)
	this.Empty(cookie.Value)
	this.True(cookie.Expires.Before(time.Now()))
}
