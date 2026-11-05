package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	setPasswordUserEmail       = "ana@x.com"
	setPasswordOldPassword     = "ana-correct-horse-battery"
	setPasswordNewPassword     = "ana-new-staple-for-the-horse"
	setPasswordOtherPassword   = "ana-another-staple-for-the-horse"
	setPasswordLinkInvalid     = "this link is no longer valid, ask your admin for a new one"
	setPasswordPasswordInvalid = "the password must have 15 to 128 characters"
	setPasswordFixedExpiry     = "2099-01-02 03:04:00+00"
	setPasswordForLine         = `<p>For ` + setPasswordUserEmail + `</p>`
	setPasswordExpiryLine      = `<p>This link works once, until <time datetime="2099-01-02T03:04:00Z">2099-01-02 03:04 UTC</time></p>`
)

var setPasswordInviteLinkInPage = regexp.MustCompile(`http://localhost:8080(/set-password\?token=([A-Za-z0-9_-]{43}))`)

func (this *IntegrationSuite) createInvitedUserWithLink(email string) (token string) {
	token, err := tokens.Random()
	this.Require().NoError(err)

	this.Require().True(repository.NewUserRepository(this.Pool).CreateInvited(context.Background(),
		&model.User{Email: email, Permission: model.PermissionWrite},
		&model.OneTimeLink{TokenHash: tokens.Hash(token), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))

	return token
}

func (this *IntegrationSuite) createResetLink(userID int64) (token string) {
	token, err := tokens.Random()
	this.Require().NoError(err)

	_, found, deactivated, _ := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(context.Background(),
		&model.OneTimeLink{TokenHash: tokens.Hash(token), UserID: userID, ExpiresAt: time.Now().Add(24 * time.Hour)},
	)
	this.Require().True(found)
	this.Require().False(deactivated)

	return token
}

func (this *IntegrationSuite) submitSetPassword(token, password string) (status int, header http.Header, body string) {
	return this.postForm("/set-password", "", "", url.Values{"token": {token}, "password": {password}})
}

func (this *IntegrationSuite) submitSetPasswordWithHtmx(token, password string) (status int, header http.Header, body string) {
	form := url.Values{"token": {token}, "password": {password}}
	req := httptest.NewRequest("POST", "/set-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("HX-Request", "true")

	return this.sendPageRequest(req, "")
}

func (this *IntegrationSuite) postSetPasswordFrom(origin, token string) int {
	form := url.Values{"token": {token}, "password": {setPasswordNewPassword}}
	req := httptest.NewRequest("POST", "/set-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	status, _, _ := this.sendPageRequest(req, "")
	return status
}

func (this *IntegrationSuite) signInStatus(email, password string) int {
	status, _, _ := this.postForm("/sign-in", "", "", url.Values{"email": {email}, "password": {password}})
	return status
}

func (this *IntegrationSuite) TestSetPassword_InvitedUserSetsAPasswordAndSignsInWithIt() {
	cookie := this.signInAsAdmin()
	_, _, form := this.getPage("/users/invite", cookie)
	status, _, invited := this.postForm("/users/invite", cookie, this.csrfTokenFrom(form), url.Values{
		"email":      {setPasswordUserEmail},
		"permission": {"read"},
	})
	this.Require().Equal(http.StatusOK, status)
	match := setPasswordInviteLinkInPage.FindStringSubmatch(invited)
	this.Require().Len(match, 3, "page shows no invite link")

	status, _, page := this.getPage(match[1], "")
	this.Require().Equal(http.StatusOK, status)
	this.Contains(page, `<form class="sheet formcard" method="post" action="/set-password">`)
	this.NotContains(page, `aria-invalid`)
	this.Contains(page, `<input type="hidden" name="token" value="`+match[2]+`">`)
	this.Contains(page, `name="password"`)
	this.NotContains(page, `action="/sign-out"`)

	status, header, _ := this.submitSetPassword(match[2], setPasswordNewPassword)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	this.NotEmpty(this.signInAsUser(setPasswordUserEmail, setPasswordNewPassword))
}

func (this *IntegrationSuite) TestSetPassword_HtmxSubmitRedirectsWithHXRedirect() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)

	status, header, _ := this.submitSetPasswordWithHtmx(token, setPasswordNewPassword)
	this.Equal(http.StatusOK, status)
	this.Equal("/sign-in", header.Get("HX-Redirect"))
	this.Empty(header.Get("HX-Push-Url"))
}

func (this *IntegrationSuite) TestSetPassword_HtmxShortPasswordAnswers400WithoutPushingTheFormURL() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)

	status, header, body := this.submitSetPasswordWithHtmx(token, "fourteen-chars")
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, setPasswordPasswordInvalid)
	this.Equal("false", header.Get("HX-Push-Url"))
}

func (this *IntegrationSuite) TestSetPassword_HtmxUsedLinkAnswers410WithoutPushingTheFormURL() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)
	status, _, _ := this.submitSetPassword(token, setPasswordNewPassword)
	this.Require().Equal(http.StatusSeeOther, status)

	status, header, body := this.submitSetPasswordWithHtmx(token, setPasswordOtherPassword)
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
	this.Equal("false", header.Get("HX-Push-Url"))
}

func (this *IntegrationSuite) TestSetPassword_ResetLinkEndsTheExistingSessionAndTheNewPasswordWorks() {
	userID := this.createActiveUser(setPasswordUserEmail, model.PermissionWrite, setPasswordOldPassword)
	cookie := this.signInAsUser(setPasswordUserEmail, setPasswordOldPassword)
	token := this.createResetLink(userID)

	status, _, _ := this.getPage("/set-password?token="+token, "")
	this.Require().Equal(http.StatusOK, status)

	status, header, _ := this.submitSetPassword(token, setPasswordNewPassword)
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	this.Equal(0, this.countRows("sessions"))
	status, header, _ = this.getPage("/profile", cookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	this.Equal(http.StatusUnauthorized, this.signInStatus(setPasswordUserEmail, setPasswordOldPassword))
	this.NotEmpty(this.signInAsUser(setPasswordUserEmail, setPasswordNewPassword))
}

func (this *IntegrationSuite) TestSetPassword_ExpiredLinkShowsTheInvalidLinkPage() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)
	_, err := this.Pool.Exec(context.Background(), `UPDATE one_time_links SET expires_at = now() - interval '1 minute'`)
	this.Require().NoError(err)

	status, _, body := this.getPage("/set-password?token="+token, "")
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
	this.NotContains(body, token)

	status, _, body = this.submitSetPassword(token, setPasswordNewPassword)
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
	this.Equal(http.StatusUnauthorized, this.signInStatus(setPasswordUserEmail, setPasswordNewPassword))
}

func (this *IntegrationSuite) TestSetPassword_UsedLinkShowsTheInvalidLinkPage() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)
	status, _, _ := this.submitSetPassword(token, setPasswordNewPassword)
	this.Require().Equal(http.StatusSeeOther, status)

	status, _, body := this.getPage("/set-password?token="+token, "")
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
}

func (this *IntegrationSuite) TestSetPassword_TwoSubmitsOfOneLinkSetThePasswordOnce() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)

	status, _, _ := this.submitSetPassword(token, setPasswordNewPassword)
	this.Require().Equal(http.StatusSeeOther, status)

	status, _, body := this.submitSetPassword(token, setPasswordOtherPassword)
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)

	this.Equal(http.StatusUnauthorized, this.signInStatus(setPasswordUserEmail, setPasswordOtherPassword))
	this.NotEmpty(this.signInAsUser(setPasswordUserEmail, setPasswordNewPassword))
}

func (this *IntegrationSuite) TestSetPassword_DeactivatedUsersLinkShowsTheInvalidLinkPage() {
	userID := this.createActiveUser(setPasswordUserEmail, model.PermissionWrite, setPasswordOldPassword)
	token := this.createResetLink(userID)
	this.setUserState(userID, model.UserDeactivated)

	status, _, body := this.getPage("/set-password?token="+token, "")
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)

	status, _, body = this.submitSetPassword(token, setPasswordNewPassword)
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)

	this.setUserState(userID, model.UserActive)
	this.Equal(http.StatusUnauthorized, this.signInStatus(setPasswordUserEmail, setPasswordNewPassword))
}

func (this *IntegrationSuite) TestSetPassword_UnknownOrMissingTokenShowsTheInvalidLinkPage() {
	for _, path := range []string{"/set-password?token=unknown", "/set-password"} {
		status, _, body := this.getPage(path, "")
		this.Equal(http.StatusGone, status, path)
		this.Contains(body, `<p role="alert">`+setPasswordLinkInvalid+`</p>`, path)
		this.Contains(body, `<a class="btn" href="/sign-in">Go to sign in</a>`, path)
	}
}

func (this *IntegrationSuite) TestSetPassword_ShortPasswordAnswers400AndKeepsTheLinkUsable() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)

	status, _, body := this.submitSetPassword(token, "fourteen-chars")
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, `autocomplete="new-password" required autofocus aria-invalid="true" aria-describedby="password-err">`)
	this.Contains(body, `<p class="err" id="password-err">`)
	this.Contains(body, `<span>`+setPasswordPasswordInvalid+`</span></p>`)
	this.Contains(body, `<input type="hidden" name="token" value="`+token+`">`)
	this.NotContains(body, "fourteen-chars")

	status, _, _ = this.getPage("/set-password?token="+token, "")
	this.Equal(http.StatusOK, status)
}

func (this *IntegrationSuite) TestSetPassword_FormShowsTheEmailAndTheLinkExpiry() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)
	_, err := this.Pool.Exec(context.Background(), `UPDATE one_time_links SET expires_at = $1`, setPasswordFixedExpiry)
	this.Require().NoError(err)

	status, _, page := this.getPage("/set-password?token="+token, "")
	this.Require().Equal(http.StatusOK, status)
	this.Contains(page, setPasswordForLine)
	this.Contains(page, setPasswordExpiryLine)

	status, _, body := this.submitSetPassword(token, "fourteen-chars")
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, setPasswordPasswordInvalid)
	this.Contains(body, setPasswordForLine)
	this.Contains(body, setPasswordExpiryLine)
}

func (this *IntegrationSuite) TestSetPassword_LinkIsCheckedBeforeThePasswordRule() {
	status, _, body := this.submitSetPassword("unknown", "short")
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
	this.NotContains(body, setPasswordPasswordInvalid)
}

func (this *IntegrationSuite) TestSetPassword_ShortPasswordOnAnExpiredLinkShowsTheInvalidLinkPage() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)
	_, err := this.Pool.Exec(context.Background(), `UPDATE one_time_links SET expires_at = now() - interval '1 minute'`)
	this.Require().NoError(err)

	status, _, body := this.submitSetPassword(token, "fourteen-chars")
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
	this.NotContains(body, setPasswordPasswordInvalid)
}

func (this *IntegrationSuite) TestSetPassword_UnknownTokenWithAValidPasswordAnswers410() {
	status, _, body := this.submitSetPassword("unknown", setPasswordNewPassword)
	this.Equal(http.StatusGone, status)
	this.Contains(body, setPasswordLinkInvalid)
}

func (this *IntegrationSuite) TestSetPassword_ForeignOrMissingOriginIsRefused() {
	token := this.createInvitedUserWithLink(setPasswordUserEmail)

	this.Equal(http.StatusForbidden, this.postSetPasswordFrom("http://evil.example", token))
	this.Equal(http.StatusForbidden, this.postSetPasswordFrom("", token))

	status, _, _ := this.getPage("/set-password?token="+token, "")
	this.Equal(http.StatusOK, status)
	this.Equal(http.StatusUnauthorized, this.signInStatus(setPasswordUserEmail, setPasswordNewPassword))
}
