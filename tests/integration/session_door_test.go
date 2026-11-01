package integration_test

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
	"github.com/guregu/null"
)

const (
	doorSessionCookieName = "bidirekt_session"
	doorSessionCookie     = "door-session-cookie"
	doorCSRFToken         = "door-csrf-token"
	doorPublicURL         = "http://localhost:8080"
	doorUnknownPage       = "/no-such-page-behind-the-door"
)

type doorRequest struct {
	method  string
	path    string
	cookie  string
	headers map[string]string
	form    string
}

func (this *IntegrationSuite) sendThroughDoor(request doorRequest) (status int, header http.Header, body string) {
	req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.form))
	if request.form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
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

func (this *IntegrationSuite) createDoorSession(userID null.Int) []byte {
	idHash := sha256.Sum256([]byte(doorSessionCookie))
	repository.NewSessionRepository(this.Pool).Create(context.Background(),
		&model.Session{IDHash: idHash[:], UserID: userID, CSRFToken: doorCSRFToken},
	)
	return idHash[:]
}

func (this *IntegrationSuite) createDoorUser(active bool) null.Int {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, permission, password_hash, active) VALUES ('ana@x.com', 'write', 'some-hash', $1) RETURNING id`,
		active,
	).Scan(&id)
	this.Require().NoError(err)
	return null.IntFrom(id)
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
	this.createDoorSession(null.Int{})

	status, _, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestSessionDoor_IdleSession_RedirectsAndIsDeleted() {
	this.createDoorSession(null.Int{})
	_, err := this.Pool.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - interval '8 hours 1 minute'`)
	this.Require().NoError(err)

	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: doorUnknownPage, cookie: doorSessionCookie})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionDoor_SessionOlderThanSevenDays_Redirects() {
	this.createDoorSession(null.Int{})
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
	this.createDoorSession(null.Int{})

	status, header, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL},
	})
	this.Equal(http.StatusForbidden, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
}

func (this *IntegrationSuite) TestSessionDoor_PostWithWrongCSRFToken_Answers403() {
	this.createDoorSession(null.Int{})

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL, "X-CSRF-Token": "not-the-token"},
	})
	this.Equal(http.StatusForbidden, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithForeignOrigin_Answers403() {
	this.createDoorSession(null.Int{})

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": "http://evil.example", "X-CSRF-Token": doorCSRFToken},
	})
	this.Equal(http.StatusForbidden, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithoutOrigin_Answers403() {
	this.createDoorSession(null.Int{})

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"X-CSRF-Token": doorCSRFToken},
	})
	this.Equal(http.StatusForbidden, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithCSRFHeaderAndOrigin_PassesTheDoor() {
	this.createDoorSession(null.Int{})

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL, "X-CSRF-Token": doorCSRFToken},
	})
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestSessionDoor_PostWithCSRFFormFieldAndOrigin_PassesTheDoor() {
	this.createDoorSession(null.Int{})

	status, _, _ := this.sendThroughDoor(doorRequest{
		method: "POST", path: doorUnknownPage, cookie: doorSessionCookie,
		headers: map[string]string{"Origin": doorPublicURL},
		form:    "csrf_token=" + doorCSRFToken,
	})
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestSessionDoor_LeavesApiHealthAndAssetsOpen() {
	for _, path := range []string{"/api/participants", "/health", "/assets/app.css"} {
		status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: path})
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

func (this *IntegrationSuite) TestSessionDoor_LookalikePrefixStaysBehindTheDoor() {
	status, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: "/sign-inside"})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestStartSession_SetsCookieThatPassesTheDoor() {
	this.Components.Server.Get("/sign-in/__start_session", func(ctx fiber.Ctx) error {
		if err := components.StartSession(ctx, null.Int{}); err != nil {
			return err
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
		components.ExpireSessionCookie(ctx)
		return ctx.SendStatus(http.StatusNoContent)
	})

	_, header, _ := this.sendThroughDoor(doorRequest{method: "GET", path: "/sign-in/__expire_session"})

	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Equal(doorSessionCookieName, cookie.Name)
	this.Empty(cookie.Value)
	this.True(cookie.Expires.Before(time.Now()))
}
