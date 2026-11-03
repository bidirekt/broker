package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/model"
)

const (
	profileUserEmail            = "ana@x.com"
	profileCurrentPassword      = "ana-correct-horse-battery"
	profileNewPassword          = "ana-new-staple-for-the-horse"
	profileCurrentPasswordWrong = "the current password is wrong"
	profilePasswordInvalid      = "the password must have 15 to 128 characters"
	profilePasswordChanged      = "password changed"
	profileAdminEmail           = "admin@example.com"
	profileAdminSecret          = "suite-admin-correct-horse"
	profileLinkInvalid          = "this link is no longer valid, ask your admin for a new one"
)

var profileResetLinkInPage = regexp.MustCompile(`http://localhost:8080/set-password\?token=([A-Za-z0-9_-]{43})`)

func (this *IntegrationSuite) openProfile(cookie string) (csrfToken string) {
	status, _, page := this.getPage("/profile", cookie)
	this.Require().Equal(http.StatusOK, status)

	return this.csrfTokenFrom(page)
}

func (this *IntegrationSuite) profilePasswordHash(userID int64) string {
	var passwordHash string
	err := this.Pool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&passwordHash)
	this.Require().NoError(err)

	return passwordHash
}

func (this *IntegrationSuite) TestUpdateProfile_PageAsksCurrentAndNewPassword() {
	this.createActiveUser(profileUserEmail, model.PermissionRead, profileCurrentPassword)
	cookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)

	status, _, body := this.getPage("/profile", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, `<form method="post" action="/profile">`)
	this.Contains(body, `name="current_password"`)
	this.Contains(body, `name="new_password"`)
	this.Contains(body, `action="/sign-out"`)
}

func (this *IntegrationSuite) TestUpdateProfile_WrongCurrentPassword_Answers400AndKeepsThePassword() {
	userID := this.createActiveUser(profileUserEmail, model.PermissionWrite, profileCurrentPassword)
	cookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)
	csrfToken := this.openProfile(cookie)
	passwordHashBefore := this.profilePasswordHash(userID)

	status, _, body := this.postForm("/profile", cookie, csrfToken, url.Values{
		"current_password": {"not-the-current-password"},
		"new_password":     {profileNewPassword},
	})
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, profileCurrentPasswordWrong)
	this.Equal(passwordHashBefore, this.profilePasswordHash(userID))
}

func (this *IntegrationSuite) TestUpdateProfile_WrongCurrentPasswordIsCheckedBeforeTheNewOne() {
	this.createActiveUser(profileUserEmail, model.PermissionWrite, profileCurrentPassword)
	cookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)
	csrfToken := this.openProfile(cookie)

	status, _, body := this.postForm("/profile", cookie, csrfToken, url.Values{
		"current_password": {"not-the-current-password"},
		"new_password":     {"short"},
	})
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, profileCurrentPasswordWrong)
	this.NotContains(body, profilePasswordInvalid)
}

func (this *IntegrationSuite) TestUpdateProfile_ShortNewPassword_Answers400AndKeepsThePassword() {
	userID := this.createActiveUser(profileUserEmail, model.PermissionWrite, profileCurrentPassword)
	cookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)
	csrfToken := this.openProfile(cookie)
	passwordHashBefore := this.profilePasswordHash(userID)

	status, _, body := this.postForm("/profile", cookie, csrfToken, url.Values{
		"current_password": {profileCurrentPassword},
		"new_password":     {"fourteen-chars"},
	})
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, profilePasswordInvalid)
	this.Equal(passwordHashBefore, this.profilePasswordHash(userID))
}

func (this *IntegrationSuite) TestUpdateProfile_Success_EndsOtherSessionsAndKeepsTheCurrentOne() {
	userID := this.createActiveUser(profileUserEmail, model.PermissionWrite, profileCurrentPassword)
	otherCookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)
	cookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)
	csrfToken := this.openProfile(cookie)

	status, _, body := this.postForm("/profile", cookie, csrfToken, url.Values{
		"current_password": {profileCurrentPassword},
		"new_password":     {profileNewPassword},
	})
	this.Equal(http.StatusOK, status)
	this.Contains(body, profilePasswordChanged)
	this.True(passwords.Verify(profileNewPassword, this.profilePasswordHash(userID)))

	status, header, _ := this.getPage("/profile", otherCookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	status, _, _ = this.getPage("/profile", cookie)
	this.Equal(http.StatusOK, status)
	this.Equal(1, this.countRows("sessions"))

	status, _, _ = this.postForm("/sign-in", "", "", url.Values{
		"email":    {profileUserEmail},
		"password": {profileCurrentPassword},
	})
	this.Equal(http.StatusUnauthorized, status)
	this.NotEmpty(this.signInAsUser(profileUserEmail, profileNewPassword))
}

func (this *IntegrationSuite) TestUpdateProfile_Success_EndsTheOpenResetLink() {
	userID := this.createActiveUser(profileUserEmail, model.PermissionWrite, profileCurrentPassword)
	adminCookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", adminCookie)
	status, _, page := this.postForm(fmt.Sprintf("/users/%d/link", userID), adminCookie, this.csrfTokenFrom(users), url.Values{})
	this.Require().Equal(http.StatusOK, status)
	match := profileResetLinkInPage.FindStringSubmatch(page)
	this.Require().Len(match, 2, "page shows no reset link")
	status, _, _ = this.getPage("/set-password?token="+match[1], "")
	this.Require().Equal(http.StatusOK, status)

	cookie := this.signInAsUser(profileUserEmail, profileCurrentPassword)
	status, _, _ = this.postForm("/profile", cookie, this.openProfile(cookie), url.Values{
		"current_password": {profileCurrentPassword},
		"new_password":     {profileNewPassword},
	})
	this.Require().Equal(http.StatusOK, status)

	status, _, body := this.getPage("/set-password?token="+match[1], "")
	this.Equal(http.StatusGone, status)
	this.Contains(body, profileLinkInvalid)
}

func (this *IntegrationSuite) TestUpdateProfile_AdminChangesItsPasswordAndSignsInWithTheNewOne() {
	cookie := this.signInAsAdmin()
	csrfToken := this.openProfile(cookie)

	status, _, body := this.postForm("/profile", cookie, csrfToken, url.Values{
		"current_password": {profileAdminSecret},
		"new_password":     {profileNewPassword},
	})
	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, profilePasswordChanged)

	this.NotEmpty(this.signInAsUser(profileAdminEmail, profileNewPassword))
}
