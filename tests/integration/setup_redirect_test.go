package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

var setupRedirectPages = []string{"/", "/sign-in", "/users", "/set-password?token=x", "/setupx"}

func (this *IntegrationSuite) startSetupRedirectApp() *components.Components {
	this.deleteSuiteWriter()
	this.deleteSuiteAdmin()

	app, err := internal.Run()
	this.Require().NoError(err)
	this.T().Cleanup(app.Pool.Close)

	return app
}

func (this *IntegrationSuite) setupRedirectSend(app *components.Components, req *http.Request) (status int, header http.Header, body string) {
	resp, err := app.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) TestSetupRedirect_EveryPageLeadsToSetupBeforeTheAdminExists() {
	app := this.startSetupRedirectApp()

	for _, path := range setupRedirectPages {
		status, header, _ := this.setupRedirectSend(app, httptest.NewRequest("GET", path, nil))
		this.Equal(http.StatusSeeOther, status, path)
		this.Equal("/setup", header.Get("Location"), path)
	}
}

func (this *IntegrationSuite) TestSetupRedirect_HtmxPagesAnswerHXRedirectToSetup() {
	app := this.startSetupRedirectApp()

	for _, path := range setupRedirectPages {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("HX-Request", "true")

		status, header, _ := this.setupRedirectSend(app, req)
		this.Equal(http.StatusOK, status, path)
		this.Equal("/setup", header.Get("HX-Redirect"), path)
		this.Empty(header.Get("Location"), path)
	}
}

func (this *IntegrationSuite) TestSetupRedirect_LeavesHealthAssetsAndApiToTheirOwnAnswers() {
	app := this.startSetupRedirectApp()

	status, _, _ := this.setupRedirectSend(app, httptest.NewRequest("GET", "/health", nil))
	this.Equal(http.StatusOK, status)

	status, header, _ := this.setupRedirectSend(app, httptest.NewRequest("GET", "/assets/app.css", nil))
	this.Equal(http.StatusOK, status)
	this.Contains(header.Get("Content-Type"), "text/css")

	signIn := httptest.NewRequest("POST", "/api/sign-in", strings.NewReader(`{"email":"admin@example.com","password":"suite-admin-correct-horse"}`))
	signIn.Header.Set("Content-Type", "application/json")
	status, _, body := this.setupRedirectSend(app, signIn)
	this.Equal(http.StatusUnauthorized, status)
	this.JSONEq(`{"message":"invalid credentials"}`, body)

	status, _, body = this.setupRedirectSend(app, httptest.NewRequest("GET", "/api/participants", nil))
	this.Equal(http.StatusUnauthorized, status)
	this.JSONEq(`{"message":"unauthenticated"}`, body)
}
