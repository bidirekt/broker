package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	signInUserEmail                 = "ana@x.com"
	signInUserPassword              = "ana-correct-horse-battery"
	signInInvalidCredentials        = "invalid email, username or password"
	signInPageContentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'"
	signInAdminUsername             = "admin"
	signInAdminPassword             = "integration-admin-password"
	signInPublicURL                 = "http://localhost:8080"
)

func (this *IntegrationSuite) signInForm(identifier, password string) url.Values {
	return url.Values{"identifier": {identifier}, "password": {password}}
}

func (this *IntegrationSuite) postSignInFrom(origin string) int {
	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(this.signInForm(signInAdminUsername, signInAdminPassword).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	status, _, _ := this.sendPageRequest(req, "")
	return status
}

func (this *IntegrationSuite) TestSignIn_PageShowsOneFormWithCSP() {
	status, header, body := this.getPage("/sign-in", "")
	this.Equal(http.StatusOK, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal(signInPageContentSecurityPolicy, header.Get("Content-Security-Policy"))
	this.Equal(1, strings.Count(body, "<form"))
	this.Contains(body, `<form method="post" action="/sign-in">`)
	this.Contains(body, "Email or username")
	this.Contains(body, "Password")
	this.NotContains(body, `action="/sign-out"`)
}

func (this *IntegrationSuite) TestSignIn_AdminLandsOnUsers() {
	cookie := this.signInAsAdmin()

	status, header, _ := this.getPage("/", cookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/users", header.Get("Location"))

	var adminSessions int
	err := this.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE user_id IS NULL`).Scan(&adminSessions)
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

func (this *IntegrationSuite) TestSignIn_UserLandsOnProfile() {
	this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)

	status, header, _ := this.postForm("/sign-in", "", "", this.signInForm(signInUserEmail, signInUserPassword))
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/", header.Get("Location"))

	cookie := this.signInAsUser(signInUserEmail, signInUserPassword)
	status, header, _ = this.getPage("/", cookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/profile", header.Get("Location"))
}

func (this *IntegrationSuite) TestSignIn_EmailMatchesIgnoringCase() {
	this.createActiveUser(signInUserEmail, model.PermissionRead, signInUserPassword)

	cookie := this.signInAsUser("ANA@X.com", signInUserPassword)
	this.NotEmpty(cookie)
}

func (this *IntegrationSuite) TestSignIn_HtmxRequestAnswersHXRedirect() {
	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(this.signInForm(signInAdminUsername, signInAdminPassword).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", signInPublicURL)
	req.Header.Set("HX-Request", "true")

	status, header, _ := this.sendPageRequest(req, "")
	this.Equal(http.StatusOK, status)
	this.Equal("/", header.Get("HX-Redirect"))
	this.Contains(header.Get("Set-Cookie"), "bidirekt_session=")
}

func (this *IntegrationSuite) TestSignIn_WrongPasswordShowsMessageAndKeepsIdentifier() {
	this.createActiveUser(signInUserEmail, model.PermissionWrite, signInUserPassword)

	status, header, body := this.postForm("/sign-in", "", "", this.signInForm(signInUserEmail, "not-the-right-password"))
	this.Equal(http.StatusUnauthorized, status)
	this.Empty(header.Get("Set-Cookie"))
	this.Contains(body, signInInvalidCredentials)
	this.Contains(body, `value="ana@x.com"`)
	this.NotContains(body, "not-the-right-password")
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSignIn_WrongAdminPasswordAnswers401() {
	status, _, body := this.postForm("/sign-in", "", "", this.signInForm(signInAdminUsername, "not-the-admin-password"))
	this.Equal(http.StatusUnauthorized, status)
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
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSignIn_HomeWithoutSessionRedirectsToSignIn() {
	status, header, _ := this.getPage("/", "")
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}
