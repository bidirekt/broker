package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

const (
	signInUserEmail                 = "ana@x.com"
	signInUserPassword              = "ana-correct-horse-battery"
	signInInvalidCredentials        = "invalid email or password"
	signInPageContentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'"
	signInSuiteAdminEmail           = "admin@example.com"
	signInSuiteAdminSecret          = "suite-admin-correct-horse"
	signInPublicURL                 = "http://localhost:8080"
)

func (this *IntegrationSuite) signInForm(email, password string) url.Values {
	return url.Values{"email": {email}, "password": {password}}
}

func (this *IntegrationSuite) postSignInFrom(origin string) int {
	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(this.signInForm(signInSuiteAdminEmail, signInSuiteAdminSecret).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	status, _, _ := this.sendPageRequest(req, "")
	return status
}

func (this *IntegrationSuite) signInAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) TestSignIn_PageShowsOneFormWithCSP() {
	status, header, body := this.getPage("/sign-in", "")
	this.Equal(http.StatusOK, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Equal(signInPageContentSecurityPolicy, header.Get("Content-Security-Policy"))
	this.Equal(1, strings.Count(body, "<form"))
	this.Contains(body, `<form class="sheet formcard" method="post" action="/sign-in">`)
	this.Contains(body, `<label for="email">Email</label>`)
	this.Contains(body, `<input class="input" id="email" name="email" type="text" inputmode="email"`)
	this.Contains(body, `<button type="submit" class="btn primary block">Sign in</button>`)
	this.NotContains(body, "Email or username")
	this.Equal(1, strings.Count(body, "username"))
	this.Contains(body, `autocomplete="username"`)
	this.Contains(body, "Password")
	this.NotContains(body, `action="/sign-out"`)
}

func (this *IntegrationSuite) TestSignIn_AdminLandsOnTheOverview() {
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage("/", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, "<h1>Overview</h1>")

	var adminSessions int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE user_id = $1`, this.adminID).Scan(&adminSessions)
	this.Require().NoError(err)
	this.Equal(1, adminSessions)
}

func (this *IntegrationSuite) TestSignIn_SignedInPagesCarryTheSessionCSRFToken() {
	cookie := this.signInAsAdmin()

	_, _, body := this.getPage("/no-such-page", cookie)

	var csrfToken string
	err := this.Pool.QueryRow(context.Background(), `SELECT csrf_token FROM sessions`).Scan(&csrfToken)
	this.Require().NoError(err)
	this.Equal(csrfToken, this.csrfTokenFrom(body))
}

func (this *IntegrationSuite) TestSignIn_UserLandsOnTheOverview() {
	this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)

	status, header, _ := this.postForm("/sign-in", "", "", this.signInForm(signInUserEmail, signInUserPassword))
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/", header.Get("Location"))

	cookie := this.signInAsUser(signInUserEmail, signInUserPassword)
	status, _, body := this.getPage("/", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, "<h1>Overview</h1>")
}

func (this *IntegrationSuite) TestSignIn_AdminNavigationLinksToUsersAndProfile() {
	cookie := this.signInAsAdmin()

	_, _, body := this.getPage("/users", cookie)

	this.Contains(body, `<a href="/users" aria-current="page">Users</a>`)
	this.Contains(body, `<a href="/profile">Profile</a>`)
}

func (this *IntegrationSuite) TestSignIn_UserNavigationLinksOnlyToProfile() {
	this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)
	cookie := this.signInAsUser(signInUserEmail, signInUserPassword)

	_, _, body := this.getPage("/profile", cookie)

	this.Contains(body, `<a href="/profile" aria-current="page">Profile</a>`)
	this.NotContains(body, `href="/users"`)
}

func (this *IntegrationSuite) TestSignIn_EmailMatchesIgnoringCase() {
	this.createActiveUser(signInUserEmail, model.PermissionRead, signInUserPassword)

	cookie := this.signInAsUser("ANA@X.com", signInUserPassword)
	this.NotEmpty(cookie)
}

func (this *IntegrationSuite) TestSignIn_HtmxRequestAnswersHXRedirect() {
	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(this.signInForm(signInSuiteAdminEmail, signInSuiteAdminSecret).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", signInPublicURL)
	req.Header.Set("HX-Request", "true")

	status, header, _ := this.sendPageRequest(req, "")
	this.Equal(http.StatusOK, status)
	this.Equal("/", header.Get("HX-Redirect"))
	this.Contains(header.Get("Set-Cookie"), "bidirekt_session=")
}

func (this *IntegrationSuite) TestSignIn_WrongPasswordShowsMessageAndKeepsEmail() {
	this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)

	status, header, body := this.postForm("/sign-in", "", "", this.signInForm(signInUserEmail, "not-the-right-password"))
	this.Equal(http.StatusUnauthorized, status)
	this.Empty(header.Get("Set-Cookie"))
	this.Contains(body, `<div class="alert" role="alert">`)
	this.Contains(body, `<span>`+signInInvalidCredentials+`</span></div>`)
	this.Contains(body, `value="ana@x.com"`)
	this.NotContains(body, "not-the-right-password")
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSignIn_AdminWithAWrongPasswordAnswers401() {
	status, _, body := this.postForm("/sign-in", "", "", this.signInForm(signInSuiteAdminEmail, "not-the-admin-password"))
	this.Equal(http.StatusUnauthorized, status)
	this.Contains(body, signInInvalidCredentials)
}

func (this *IntegrationSuite) TestSignIn_ANameWithoutAnAtAnswersLikeAWrongPassword() {
	status, header, body := this.postForm("/sign-in", "", "", this.signInForm("admin", signInSuiteAdminSecret))
	this.Equal(http.StatusUnauthorized, status)
	this.Empty(header.Get("Set-Cookie"))
	this.Contains(body, signInInvalidCredentials)
}

func (this *IntegrationSuite) TestSignIn_InvitedUserAnswers401() {
	userID := this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)
	this.setUserState(userID, model.UserInvited)

	status, _, body := this.postForm("/sign-in", "", "", this.signInForm(signInUserEmail, signInUserPassword))
	this.Equal(http.StatusUnauthorized, status)
	this.Contains(body, signInInvalidCredentials)
}

func (this *IntegrationSuite) TestSignIn_DeactivatedUserAnswers401() {
	userID := this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)
	this.setUserState(userID, model.UserDeactivated)

	status, _, body := this.postForm("/sign-in", "", "", this.signInForm(signInUserEmail, signInUserPassword))
	this.Equal(http.StatusUnauthorized, status)
	this.Contains(body, signInInvalidCredentials)
}

func (this *IntegrationSuite) TestSignIn_UserDeactivatedWhileSigningInAnswers401WithoutASession() {
	ctx := context.Background()
	userID := this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)

	deactivation, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = deactivation.Rollback(ctx) }()
	_, err = deactivation.Exec(ctx, `UPDATE users SET active = false, updated_at = now() WHERE id = $1`, userID)
	this.Require().NoError(err)

	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(this.signInForm(signInUserEmail, signInUserPassword).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", signInPublicURL)
	req.Header.Set("X-Forwarded-For", this.clientIP)

	type signInResult struct {
		response *http.Response
		err      error
	}
	finished := make(chan signInResult, 1)
	go func() {
		response, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
		finished <- signInResult{response: response, err: err}
	}()
	this.signInAwaitLockWaiters(1)

	this.Require().NoError(deactivation.Commit(ctx))
	result := <-finished
	this.Require().NoError(result.err)
	defer func() { _ = result.response.Body.Close() }()
	body, err := io.ReadAll(result.response.Body)
	this.Require().NoError(err)

	this.Equal(http.StatusUnauthorized, result.response.StatusCode)
	this.Contains(string(body), signInInvalidCredentials)
	this.Empty(result.response.Header.Get("Set-Cookie"))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSignIn_UnknownEmailAnswers401() {
	status, _, body := this.postForm("/sign-in", "", "", this.signInForm("nobody@x.com", signInUserPassword))
	this.Equal(http.StatusUnauthorized, status)
	this.Contains(body, signInInvalidCredentials)
}

func (this *IntegrationSuite) TestSignIn_ForeignOriginAnswers403() {
	this.Equal(http.StatusForbidden, this.postSignInFrom("http://evil.example"))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSignIn_MissingOriginAnswers403() {
	this.Equal(http.StatusForbidden, this.postSignInFrom(""))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSignIn_ExpiredSessionRedirectsFromHomeToSignIn() {
	cookie := this.signInAsAdmin()
	_, err := this.Pool.Exec(context.Background(), `UPDATE sessions SET last_seen_at = now() - interval '9 hours'`)
	this.Require().NoError(err)

	status, header, _ := this.getPage("/", cookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2F", header.Get("Location"))
}

func (this *IntegrationSuite) TestSignIn_HomeWithoutSessionRedirectsToSignIn() {
	status, header, _ := this.getPage("/", "")
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2F", header.Get("Location"))
}
