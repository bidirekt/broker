package integration_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	signInNextEmail         = "nina@x.com"
	signInNextPassword      = "nina-correct-horse-battery"
	signInNextWrongPassword = "nina-wrong-horse-battery"
	signInNextPage          = "/profile/api-keys/new"
	signInNextSignInPath    = "/sign-in?next=%2Fprofile%2Fapi-keys%2Fnew"
	signInNextHiddenField   = `<input type="hidden" name="next" value="/profile/api-keys/new">`
	signInNextHomeField     = `<input type="hidden" name="next" value="/">`
)

func (this *IntegrationSuite) signInNextForm(password, next string) url.Values {
	return url.Values{"email": {signInNextEmail}, "password": {password}, "next": {next}}
}

func (this *IntegrationSuite) TestSignInNext_GetWithoutSessionCarriesThePageToSignIn() {
	status, header, _ := this.getPage(signInNextPage, "")
	this.Equal(http.StatusSeeOther, status)
	this.Equal(signInNextSignInPath, header.Get("Location"))

	status, header, _ = this.getPage("/participants?page=2", "")
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in?next=%2Fparticipants%3Fpage%3D2", header.Get("Location"))
}

func (this *IntegrationSuite) TestSignInNext_HeadWithoutSessionCarriesThePageToSignIn() {
	status, header, _ := this.sendPageRequest(httptest.NewRequest("HEAD", signInNextPage, nil), "")
	this.Equal(http.StatusSeeOther, status)
	this.Equal(signInNextSignInPath, header.Get("Location"))
}

func (this *IntegrationSuite) TestSignInNext_PostWithoutSessionGoesToSignInWithoutNext() {
	status, header, _ := this.postForm(signInNextPage, "", "", url.Values{"name": {"ci"}})
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestSignInNext_SignInReturnsToThePageEvenAfterAWrongPassword() {
	this.createActiveUser(signInNextEmail, model.PermissionWrite, signInNextPassword)

	status, _, body := this.getPage(signInNextSignInPath, "")
	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, signInNextHiddenField)

	status, _, body = this.postForm("/sign-in", "", "", this.signInNextForm(signInNextWrongPassword, signInNextPage))
	this.Require().Equal(http.StatusUnauthorized, status)
	this.Contains(body, signInNextHiddenField)

	status, header, _ := this.postForm("/sign-in", "", "", this.signInNextForm(signInNextPassword, signInNextPage))
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal(signInNextPage, header.Get("Location"))

	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	status, _, _ = this.getPage(signInNextPage, cookie.Value)
	this.Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) TestSignInNext_HtmxSignInReturnsToThePageWithHXRedirect() {
	this.createActiveUser(signInNextEmail, model.PermissionWrite, signInNextPassword)

	req := httptest.NewRequest("POST", "/sign-in", strings.NewReader(this.signInNextForm(signInNextPassword, signInNextPage).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", suitePublicURL)
	req.Header.Set("HX-Request", "true")

	status, header, _ := this.sendPageRequest(req, "")
	this.Equal(http.StatusOK, status)
	this.Equal(signInNextPage, header.Get("HX-Redirect"))
}

func (this *IntegrationSuite) TestSignInNext_TooManyAttemptsKeepsTheNext() {
	this.createActiveUser(signInNextEmail, model.PermissionWrite, signInNextPassword)

	for range 10 {
		status, _, _ := this.postForm("/sign-in", "", "", this.signInNextForm(signInNextWrongPassword, signInNextPage))
		this.Require().Equal(http.StatusUnauthorized, status)
	}

	status, _, body := this.postForm("/sign-in", "", "", this.signInNextForm(signInNextPassword, signInNextPage))
	this.Equal(http.StatusTooManyRequests, status)
	this.Contains(body, signInNextHiddenField)
}

func (this *IntegrationSuite) TestSignInNext_AnythingButALocalPathGoesHome() {
	this.createActiveUser(signInNextEmail, model.PermissionWrite, signInNextPassword)

	for _, next := range []string{"//evil.com", "https://evil.com", `/\evil.com`, "/profile\x01", "/a//b", "evil.com", ""} {
		status, _, body := this.getPage("/sign-in?next="+url.QueryEscape(next), "")
		this.Require().Equal(http.StatusOK, status, next)
		this.Contains(body, signInNextHomeField, next)

		status, header, _ := this.postForm("/sign-in", "", "", this.signInNextForm(signInNextPassword, next))
		this.Require().Equal(http.StatusSeeOther, status, next)
		this.Equal("/", header.Get("Location"), next)
	}
}
