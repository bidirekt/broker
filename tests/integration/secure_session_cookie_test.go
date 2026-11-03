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
	"github.com/gofiber/fiber/v3"
)

const (
	secureCookiePublicURL   = "https://broker.example.com"
	secureCookieName        = "__Host-bidirekt_session"
	secureCookieAdminEmail  = "admin@example.com"
	secureCookieAdminSecret = "suite-admin-correct-horse"
)

func (this *IntegrationSuite) startSecureCookieApp() *components.Components {
	this.T().Setenv("BIDIREKT_PUBLIC_URL", secureCookiePublicURL)

	app, err := internal.Run()
	this.Require().NoError(err)
	this.T().Cleanup(app.Pool.Close)

	return app
}

func (this *IntegrationSuite) secureCookieSend(app *components.Components, req *http.Request) (status int, header http.Header, body string) {
	resp, err := app.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) TestSecureSessionCookie_HttpsPublicURLSignsInWithASecureHostCookieThatOpensPages() {
	app := this.startSecureCookieApp()

	form := url.Values{"email": {secureCookieAdminEmail}, "password": {secureCookieAdminSecret}}
	signIn := httptest.NewRequest("POST", "/sign-in", strings.NewReader(form.Encode()))
	signIn.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	signIn.Header.Set("Origin", secureCookiePublicURL)
	status, header, _ := this.secureCookieSend(app, signIn)
	this.Require().Equal(http.StatusSeeOther, status)

	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Equal(secureCookieName, cookie.Name)
	this.True(cookie.Secure)
	this.Equal("/", cookie.Path)
	this.Empty(cookie.Domain)

	page := httptest.NewRequest("GET", "/users", nil)
	page.AddCookie(&http.Cookie{Name: secureCookieName, Value: cookie.Value})
	status, header, _ = this.secureCookieSend(app, page)
	this.Equal(http.StatusOK, status)
	this.Empty(header.Get("Location"))
}
