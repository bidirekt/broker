package integration_test

import (
	"encoding/json"
	"fmt"
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
	rateLimitPublicURL      = "http://localhost:8080"
	rateLimitAdminUsername  = "admin"
	rateLimitAdminPassword  = "integration-admin-password"
	rateLimitAnaEmail       = "ana@x.com"
	rateLimitBiaEmail       = "bia@x.com"
	rateLimitPassword       = "rate-limit-correct-horse"
	rateLimitWrongPassword  = "rate-limit-wrong-horse"
	rateLimitTooManyMessage = "too many attempts, try again later"
	rateLimitTooManyBody    = `{"message":"too many attempts, try again later"}`
)

func (this *IntegrationSuite) startRateLimitedApp() *components.Components {
	app, err := internal.Run()
	this.Require().NoError(err)
	this.T().Cleanup(app.Pool.Close)

	return app
}

func (this *IntegrationSuite) rateLimitSend(app *components.Components, req *http.Request, forwardedFor string) (status int, body string) {
	req.Header.Set("X-Forwarded-For", forwardedFor)

	resp, err := app.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, string(bytes)
}

func (this *IntegrationSuite) rateLimitPageSignIn(app *components.Components, forwardedFor, identifier, password string) (status int, body string) {
	form := url.Values{"identifier": {identifier}, "password": {password}}
	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", rateLimitPublicURL)
	req.Header.Set("Accept", "text/html")

	return this.rateLimitSend(app, req, forwardedFor)
}

func (this *IntegrationSuite) rateLimitApiSignIn(app *components.Components, forwardedFor, email, password string) (status int, body string) {
	requestBody, err := json.Marshal(map[string]string{"email": email, "password": password})
	this.Require().NoError(err)
	req := httptest.NewRequest("POST", "/api/sign-in", strings.NewReader(string(requestBody)))
	req.Header.Set("Content-Type", "application/json")

	return this.rateLimitSend(app, req, forwardedFor)
}

func (this *IntegrationSuite) TestSignInRateLimit_EleventhFailureForOneAccountIsRefusedOnEitherDoor() {
	app := this.startRateLimitedApp()
	this.createActiveUser(rateLimitAnaEmail, model.PermissionWrite, rateLimitPassword)
	this.createActiveUser(rateLimitBiaEmail, model.PermissionWrite, rateLimitPassword)
	const clientIP = "203.0.113.10"

	for range 5 {
		status, _ := this.rateLimitPageSignIn(app, clientIP, rateLimitAnaEmail, rateLimitWrongPassword)
		this.Require().Equal(http.StatusUnauthorized, status)
		status, _ = this.rateLimitApiSignIn(app, clientIP, "ANA@x.com", rateLimitWrongPassword)
		this.Require().Equal(http.StatusUnauthorized, status)
	}

	status, body := this.rateLimitPageSignIn(app, clientIP, rateLimitAnaEmail, rateLimitPassword)
	this.Equal(http.StatusTooManyRequests, status)
	this.Contains(body, rateLimitTooManyMessage)
	this.Contains(body, "<html")

	status, body = this.rateLimitApiSignIn(app, clientIP, rateLimitAnaEmail, rateLimitPassword)
	this.Equal(http.StatusTooManyRequests, status)
	this.JSONEq(rateLimitTooManyBody, body)

	status, _ = this.rateLimitPageSignIn(app, clientIP, rateLimitBiaEmail, rateLimitPassword)
	this.Equal(http.StatusSeeOther, status)
	status, _ = this.rateLimitApiSignIn(app, clientIP, rateLimitBiaEmail, rateLimitPassword)
	this.Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) TestSignInRateLimit_FiftyFirstFailureFromOneIPIsRefused() {
	app := this.startRateLimitedApp()
	const clientIP = "203.0.113.20"

	for account := range 5 {
		for attempt := range 10 {
			identifier := fmt.Sprintf("intruder-%d", account)
			signIn := this.rateLimitPageSignIn
			if attempt%2 == 1 {
				signIn = this.rateLimitApiSignIn
			}
			status, _ := signIn(app, clientIP, identifier, rateLimitWrongPassword)
			this.Require().Equal(http.StatusUnauthorized, status)
		}
	}

	status, _ := this.rateLimitPageSignIn(app, clientIP, rateLimitAdminUsername, rateLimitAdminPassword)
	this.Equal(http.StatusTooManyRequests, status)

	status, _ = this.rateLimitPageSignIn(app, "203.0.113.21", rateLimitAdminUsername, rateLimitAdminPassword)
	this.Equal(http.StatusSeeOther, status)
}

func (this *IntegrationSuite) TestSignInRateLimit_SuccessfulSignInsNeverCount() {
	app := this.startRateLimitedApp()
	this.createActiveUser(rateLimitAnaEmail, model.PermissionWrite, rateLimitPassword)
	const clientIP = "203.0.113.30"

	for range 6 {
		status, _ := this.rateLimitPageSignIn(app, clientIP, rateLimitAnaEmail, rateLimitPassword)
		this.Require().Equal(http.StatusSeeOther, status)
		status, _ = this.rateLimitApiSignIn(app, clientIP, rateLimitAnaEmail, rateLimitPassword)
		this.Require().Equal(http.StatusOK, status)
	}

	for range 10 {
		status, _ := this.rateLimitPageSignIn(app, clientIP, rateLimitAnaEmail, rateLimitWrongPassword)
		this.Require().Equal(http.StatusUnauthorized, status)
	}

	status, _ := this.rateLimitPageSignIn(app, clientIP, rateLimitAnaEmail, rateLimitWrongPassword)
	this.Equal(http.StatusTooManyRequests, status)
}

func (this *IntegrationSuite) TestSignInRateLimit_SpoofedLeftEntryOfForwardedForDoesNotChangeTheKey() {
	app := this.startRateLimitedApp()
	const clientIP = "203.0.113.40"

	for attempt := range 10 {
		status, _ := this.rateLimitPageSignIn(app, fmt.Sprintf("192.0.2.%d, %s", attempt, clientIP), rateLimitAdminUsername, rateLimitWrongPassword)
		this.Require().Equal(http.StatusUnauthorized, status)
	}

	status, _ := this.rateLimitPageSignIn(app, "198.51.100.7, "+clientIP, rateLimitAdminUsername, rateLimitWrongPassword)
	this.Equal(http.StatusTooManyRequests, status)

	status, _ = this.rateLimitPageSignIn(app, clientIP+", 203.0.113.41", rateLimitAdminUsername, rateLimitWrongPassword)
	this.Equal(http.StatusUnauthorized, status)
}

func (this *IntegrationSuite) TestSignInRateLimit_ForwardedForFromAnUntrustedSourceIsIgnored() {
	this.T().Setenv("BIDIREKT_TRUSTED_PROXIES", "192.0.2.1")
	app := this.startRateLimitedApp()

	for attempt := range 10 {
		status, _ := this.rateLimitPageSignIn(app, fmt.Sprintf("203.0.113.%d", 50+attempt), rateLimitAdminUsername, rateLimitWrongPassword)
		this.Require().Equal(http.StatusUnauthorized, status)
	}

	status, _ := this.rateLimitPageSignIn(app, "203.0.113.99", rateLimitAdminUsername, rateLimitWrongPassword)
	this.Equal(http.StatusTooManyRequests, status)
}
