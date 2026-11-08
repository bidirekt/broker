package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	profileLimitPublicURL       = "http://localhost:8080"
	profileLimitCookieName      = "bidirekt_session"
	profileLimitClientIP        = "203.0.113.60"
	profileLimitAnaEmail        = "ana@x.com"
	profileLimitBiaEmail        = "bia@x.com"
	profileLimitPassword        = "profile-limit-correct-horse"
	profileLimitWrongPassword   = "profile-limit-wrong-horse"
	profileLimitNewPassword     = "profile-limit-new-staple-horse"
	profileLimitTooManyMessage  = "too many attempts, try again later"
	profileLimitChanged         = "password changed"
	profileLimitReadDescription = "Sees every data screen and their own profile; runs can-i-deploy and validate."
)

func (this *IntegrationSuite) startProfileLimitedApp() *components.Components {
	app, err := internal.Run()
	this.Require().NoError(err)
	this.T().Cleanup(app.Pool.Close)

	return app
}

func (this *IntegrationSuite) profileLimitCreateAPIKey(userID int64, name string) {
	this.Require().True(repository.NewAPIKeyRepository(this.Pool).Create(context.Background(), &model.APIKey{
		UserID:        userID,
		KeyHash:       tokens.Hash(name),
		DisplayPrefix: "bdk_" + name,
		Name:          name,
		Permission:    model.PermissionRead,
	}))
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

func (this *IntegrationSuite) profileLimitChangePassword(app *components.Components, sessionCookie, csrfToken, currentPassword string) (status int, header http.Header, body string) {
	return this.profileLimitPostForm(app, "/profile", sessionCookie, csrfToken, url.Values{
		"current_password": {currentPassword},
		"new_password":     {profileLimitNewPassword},
	})
}

func (this *IntegrationSuite) TestUpdateProfileRateLimit_EleventhAttemptAfterTenWrongCurrentPasswordsIsRefusedForThatUserOnly() {
	app := this.startProfileLimitedApp()
	anaID := this.createActiveUser(profileLimitAnaEmail, model.PermissionRead, profileLimitPassword)
	biaID := this.createActiveUser(profileLimitBiaEmail, model.PermissionRead, profileLimitPassword)
	this.profileLimitCreateAPIKey(anaID, "anaKey01")
	this.profileLimitCreateAPIKey(biaID, "biaKey01")
	anaCookie, anaCSRFToken := this.profileLimitSignIn(app, profileLimitAnaEmail)
	biaCookie, biaCSRFToken := this.profileLimitSignIn(app, profileLimitBiaEmail)

	for range 10 {
		status, _, _ := this.profileLimitChangePassword(app, anaCookie, anaCSRFToken, profileLimitWrongPassword)
		this.Require().Equal(http.StatusBadRequest, status)
	}

	status, _, body := this.profileLimitChangePassword(app, anaCookie, anaCSRFToken, profileLimitPassword)
	this.Equal(http.StatusTooManyRequests, status)
	this.Contains(body, `<div class="alert" role="alert">`)
	this.Contains(body, "<span>"+profileLimitTooManyMessage+"</span>")
	this.Contains(body, `<form class="sheet formcard" method="post" action="/profile">`)
	this.Contains(body, `<div class="mono">`+profileLimitAnaEmail+`</div>`)
	this.Contains(body, `<span class="badge perm">read</span>`)
	this.Contains(body, profileLimitReadDescription)
	this.Contains(body, `<td data-l="Name">anaKey01</td>`)
	this.NotContains(body, "biaKey01")

	status, header, _ := this.profileLimitChangePassword(app, biaCookie, biaCSRFToken, profileLimitPassword)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/profile", header.Get("Location"))

	status, _, body = this.profileLimitSend(app, httptest.NewRequest("GET", "/profile", nil), biaCookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, "<span>"+profileLimitChanged+"</span>")
}
