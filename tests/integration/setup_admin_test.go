package integration_test

import (
	"context"
	"encoding/json"
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
	setupAdminEmail         = "first-admin@x.com"
	setupAdminPassword      = "first-admin-correct-horse"
	setupAdminPublicURL     = "http://localhost:8080"
	setupAdminForeignOrigin = "http://evil.example"
	setupAdminCookieName    = "bidirekt_session"
	setupAdminEmailInvalid  = "invalid email"
	setupAdminPasswordShort = "the password must have 15 to 128 characters"
)

func (this *IntegrationSuite) startSetupAdminApp() *components.Components {
	this.deleteSuiteWriter()
	this.deleteSuiteAdmin()

	app, err := internal.Run()
	this.Require().NoError(err)
	this.T().Cleanup(app.Pool.Close)

	return app
}

func (this *IntegrationSuite) setupAdminSend(app *components.Components, req *http.Request) (status int, header http.Header, body string) {
	resp, err := app.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func setupAdminFormRequest(path, origin string, form url.Values) *http.Request {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Origin", origin)

	return req
}

func (this *IntegrationSuite) setupAdminPost(app *components.Components, email, password string) (status int, header http.Header, body string) {
	return this.setupAdminSend(app, setupAdminFormRequest("/setup", setupAdminPublicURL, url.Values{"email": {email}, "password": {password}}))
}

func (this *IntegrationSuite) setupAdminGet(app *components.Components, path, cookie string) (status int, header http.Header, body string) {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Accept", "text/html")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: setupAdminCookieName, Value: cookie})
	}

	return this.setupAdminSend(app, req)
}

func (this *IntegrationSuite) setupAdminCookieFrom(header http.Header) string {
	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Require().Equal(setupAdminCookieName, cookie.Name)

	return cookie.Value
}

func (this *IntegrationSuite) setupAdminCount() int {
	var admins int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE permission = 'admin'`).Scan(&admins)
	this.Require().NoError(err)

	return admins
}

func (this *IntegrationSuite) setupAdminAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) TestSetupAdmin_PageAsksForTheAdminsEmailAndPasswordWithoutNavigation() {
	app := this.startSetupAdminApp()

	status, header, body := this.setupAdminGet(app, "/setup", "")

	this.Equal(http.StatusOK, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<div class="auth-brand">
  <span class="mark big" aria-hidden="true">🤝</span>
  <h1>Create the admin</h1>`)
	this.Contains(body, `<form class="sheet formcard" method="post" action="/setup">`)
	this.Contains(body, `<input class="input" id="email" name="email" type="text" inputmode="email" autocomplete="email" value="" required autofocus>`)
	this.Contains(body, `<input class="input" id="password" name="password" type="password"`)
	this.NotContains(body, `aria-invalid`)
	this.NotContains(body, `class="nav"`)
	this.NotContains(body, `action="/sign-out"`)
}

func (this *IntegrationSuite) TestSetupAdmin_CreatesTheAdminAndSignsItIn() {
	app := this.startSetupAdminApp()

	status, header, _ := this.setupAdminPost(app, setupAdminEmail, setupAdminPassword)
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/", header.Get("Location"))
	cookie := this.setupAdminCookieFrom(header)

	status, _, body := this.setupAdminGet(app, "/users", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, "<tr data-state=\"active\">\n        <td data-l=\"Email\"><span class=\"mono\">"+setupAdminEmail+"</span></td>\n        <td data-l=\"Permission\"><span class=\"badge perm\">admin</span></td>")
	this.Equal(1, this.setupAdminCount())
}

func (this *IntegrationSuite) TestSetupAdmin_TheNewAdminSignsInOnTheSignInPageAndTheApi() {
	app := this.startSetupAdminApp()
	status, _, _ := this.setupAdminPost(app, setupAdminEmail, setupAdminPassword)
	this.Require().Equal(http.StatusSeeOther, status)

	status, header, _ := this.setupAdminSend(app, setupAdminFormRequest("/sign-in", setupAdminPublicURL,
		url.Values{"email": {setupAdminEmail}, "password": {setupAdminPassword}}))
	this.Require().Equal(http.StatusSeeOther, status)

	status, _, body := this.setupAdminGet(app, "/", this.setupAdminCookieFrom(header))
	this.Equal(http.StatusOK, status)
	this.Contains(body, "<h1>Overview</h1>")

	apiSignIn := httptest.NewRequest("POST", "/api/sign-in",
		strings.NewReader(`{"email":"`+setupAdminEmail+`","password":"`+setupAdminPassword+`"}`))
	apiSignIn.Header.Set("Content-Type", "application/json")
	status, _, body = this.setupAdminSend(app, apiSignIn)
	this.Require().Equal(http.StatusOK, status, body)

	var response struct {
		Token string `json:"token"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))
	this.NotEmpty(response.Token)
}

func (this *IntegrationSuite) TestSetupAdmin_AnswersNotFoundOnceTheAdminExistsAndPagesStopLeadingToIt() {
	app := this.startSetupAdminApp()
	status, _, _ := this.setupAdminPost(app, setupAdminEmail, setupAdminPassword)
	this.Require().Equal(http.StatusSeeOther, status)

	status, _, _ = this.setupAdminGet(app, "/setup", "")
	this.Equal(http.StatusNotFound, status)

	status, _, _ = this.setupAdminPost(app, "second-admin@x.com", setupAdminPassword)
	this.Equal(http.StatusNotFound, status)

	status, _, _ = this.setupAdminGet(app, "/sign-in", "")
	this.Equal(http.StatusOK, status)

	status, header, _ := this.setupAdminGet(app, "/", "")
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	this.Equal(1, this.setupAdminCount())
}

func (this *IntegrationSuite) TestSetupAdmin_SubPathsOfSetupAreNotRouted() {
	app := this.startSetupAdminApp()

	for _, path := range []string{"/setup/", "/setup/x"} {
		status, header, _ := this.setupAdminGet(app, path, "")
		this.Equal(http.StatusNotFound, status, path)
		this.Empty(header.Get("Location"), path)
	}

	status, _, _ := this.setupAdminPost(app, setupAdminEmail, setupAdminPassword)
	this.Require().Equal(http.StatusSeeOther, status)

	for _, path := range []string{"/setup/", "/setup/x"} {
		status, _, _ = this.setupAdminGet(app, path, "")
		this.Equal(http.StatusNotFound, status, path)
	}
}

func (this *IntegrationSuite) TestSetupAdmin_RefusesAnInvalidEmailOrAFourteenCharacterPasswordWithTheForm() {
	app := this.startSetupAdminApp()

	status, _, body := this.setupAdminPost(app, "admin.x.com", setupAdminPassword)
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, `value="admin.x.com" required autofocus aria-invalid="true" aria-describedby="email-err">`)
	this.Contains(body, `<p class="err" id="email-err">`)
	this.Contains(body, `<span>`+setupAdminEmailInvalid+`</span></p>`)
	this.NotContains(body, `password-err`)

	status, _, body = this.setupAdminPost(app, setupAdminEmail, "fourteen-chars")
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, `autocomplete="new-password" required autofocus aria-invalid="true" aria-describedby="password-err">`)
	this.Contains(body, `<p class="err" id="password-err">`)
	this.Contains(body, `<span>`+setupAdminPasswordShort+`</span></p>`)
	this.NotContains(body, `email-err`)
	this.Contains(body, `value="`+setupAdminEmail+`" required>`)

	this.Equal(0, this.setupAdminCount())
}

func (this *IntegrationSuite) TestSetupAdmin_ASetupThatLosesTheRaceGoesToSignInAndOneAdminExists() {
	ctx := context.Background()
	app := this.startSetupAdminApp()

	concurrentSetup, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = concurrentSetup.Rollback(ctx) }()
	_, err = concurrentSetup.Exec(ctx, `INSERT INTO users (email, permission, password_hash) VALUES ('other-admin@x.com', 'admin', 'some-hash')`)
	this.Require().NoError(err)

	type setupResult struct {
		response *http.Response
		err      error
	}
	finished := make(chan setupResult, 1)
	req := setupAdminFormRequest("/setup", setupAdminPublicURL, url.Values{"email": {setupAdminEmail}, "password": {setupAdminPassword}})
	go func() {
		response, err := app.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
		finished <- setupResult{response: response, err: err}
	}()
	this.setupAdminAwaitLockWaiters(1)

	this.Require().NoError(concurrentSetup.Commit(ctx))
	result := <-finished
	this.Require().NoError(result.err)
	defer func() { _ = result.response.Body.Close() }()

	this.Equal(http.StatusSeeOther, result.response.StatusCode)
	this.Equal("/sign-in", result.response.Header.Get("Location"))
	this.Empty(result.response.Header.Get("Set-Cookie"))
	this.Equal(1, this.setupAdminCount())
}

func (this *IntegrationSuite) TestSetupAdmin_ForeignOriginIsRefused() {
	app := this.startSetupAdminApp()

	status, _, _ := this.setupAdminSend(app, setupAdminFormRequest("/setup", setupAdminForeignOrigin,
		url.Values{"email": {setupAdminEmail}, "password": {setupAdminPassword}}))

	this.Equal(http.StatusForbidden, status)
	this.Equal(0, this.setupAdminCount())
}
