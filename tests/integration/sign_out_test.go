package integration_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"
)

const (
	signOutPageBehindTheDoor = "/no-such-page-behind-the-sign-out-door"
	signOutForeignOrigin     = "http://evil.example"
)

func (this *IntegrationSuite) signOutSession() (cookie, csrfToken string) {
	cookie = this.signInAsAdmin()
	_, _, page := this.getPage(signOutPageBehindTheDoor, cookie)

	return cookie, this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) TestSignOut_DeletesTheSessionAndExpiresTheCookie() {
	cookie, csrfToken := this.signOutSession()

	status, header, _ := this.postForm("/sign-out", cookie, csrfToken, url.Values{})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	expired, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Equal("bidirekt_session", expired.Name)
	this.Empty(expired.Value)
	this.True(expired.Expires.Before(time.Now()))

	this.Equal(0, this.countRows("sessions"))

	status, header, _ = this.getPage(signOutPageBehindTheDoor, cookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSignOut_WithoutCSRFToken_Answers403AndKeepsTheSession() {
	cookie, _ := this.signOutSession()

	status, _, _ := this.postForm("/sign-out", cookie, "", url.Values{})
	this.Equal(http.StatusForbidden, status)

	this.Equal(1, this.countRows("sessions"))
	status, _, _ = this.getPage(signOutPageBehindTheDoor, cookie)
	this.Equal(http.StatusNotFound, status)
}

func (this *IntegrationSuite) TestSignOut_FromForeignOrigin_Answers403AndKeepsTheSession() {
	cookie, csrfToken := this.signOutSession()

	req := httptest.NewRequest("POST", "/sign-out", nil)
	req.Header.Set("Origin", signOutForeignOrigin)
	req.Header.Set("X-CSRF-Token", csrfToken)
	status, _, _ := this.sendPageRequest(req, cookie)
	this.Equal(http.StatusForbidden, status)

	this.Equal(1, this.countRows("sessions"))
	status, _, _ = this.getPage(signOutPageBehindTheDoor, cookie)
	this.Equal(http.StatusNotFound, status)
}
