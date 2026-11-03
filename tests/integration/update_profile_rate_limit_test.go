package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

const (
	profileLimitPublicURL      = "http://localhost:8080"
	profileLimitCookieName     = "bidirekt_session"
	profileLimitClientIP       = "203.0.113.60"
	profileLimitAnaEmail       = "ana@x.com"
	profileLimitBiaEmail       = "bia@x.com"
	profileLimitPassword       = "profile-limit-correct-horse"
	profileLimitWrongPassword  = "profile-limit-wrong-horse"
	profileLimitNewPassword    = "profile-limit-new-staple-horse"
	profileLimitTooManyMessage = "too many attempts, try again later"
	profileLimitChanged        = "password changed"
)

func (this *IntegrationSuite) startProfileLimitedApp() *components.Components {
	app, err := internal.Run()
	this.Require().NoError(err)
	this.T().Cleanup(app.Pool.Close)

	return app
}

func (this *IntegrationSuite) profileLimitSend(app *components.Components, req *http.Request, sessionCookie string) (status int, header http.Header, body string) {
	req.Header.Set("Accept", "text/html")
	req.Header.Set("X-Forwarded-For", profileLimitClientIP)
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: profileLimitCookieName, Value: sessionCookie})
	}

	resp, err := app.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) profileLimitPostForm(app *components.Components, path, sessionCookie, csrfToken string, form url.Values) (status int, header http.Header, body string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", profileLimitPublicURL)
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	return this.profileLimitSend(app, req, sessionCookie)
}

func (this *IntegrationSuite) profileLimitSignIn(app *components.Components, email string) (sessionCookie, csrfToken string) {
	status, header, _ := this.profileLimitPostForm(app, "/sign-in", "", "", url.Values{"email": {email}, "password": {profileLimitPassword}})
	this.Require().Equal(http.StatusSeeOther, status)

	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Require().Equal(profileLimitCookieName, cookie.Name)

	status, _, page := this.profileLimitSend(app, httptest.NewRequest("GET", "/profile", nil), cookie.Value)
	this.Require().Equal(http.StatusOK, status)

	return cookie.Value, this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) profileLimitChangePassword(app *components.Components, sessionCookie, csrfToken, currentPassword string) (status int, body string) {
	status, _, body = this.profileLimitPostForm(app, "/profile", sessionCookie, csrfToken, url.Values{
		"current_password": {currentPassword},
		"new_password":     {profileLimitNewPassword},
	})

	return status, body
}

func (this *IntegrationSuite) TestUpdateProfileRateLimit_EleventhAttemptAfterTenWrongCurrentPasswordsIsRefusedForThatUserOnly() {
	app := this.startProfileLimitedApp()
	this.createActiveUser(profileLimitAnaEmail, model.PermissionRead, profileLimitPassword)
	this.createActiveUser(profileLimitBiaEmail, model.PermissionRead, profileLimitPassword)
	anaCookie, anaCSRFToken := this.profileLimitSignIn(app, profileLimitAnaEmail)
	biaCookie, biaCSRFToken := this.profileLimitSignIn(app, profileLimitBiaEmail)

	for range 10 {
		status, _ := this.profileLimitChangePassword(app, anaCookie, anaCSRFToken, profileLimitWrongPassword)
		this.Require().Equal(http.StatusBadRequest, status)
	}

	status, body := this.profileLimitChangePassword(app, anaCookie, anaCSRFToken, profileLimitPassword)
	this.Equal(http.StatusTooManyRequests, status)
	this.Contains(body, profileLimitTooManyMessage)
	this.Contains(body, "<html")

	status, body = this.profileLimitChangePassword(app, biaCookie, biaCSRFToken, profileLimitPassword)
	this.Equal(http.StatusOK, status)
	this.Contains(body, profileLimitChanged)
}
