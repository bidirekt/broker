package integration_test

import (
	"context"
	"net/http"
	"net/url"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
)

const (
	profileUserEmail            = "ana@x.com"
	profileCurrentPassword      = "ana-correct-horse-battery"
	profileNewPassword          = "ana-new-staple-for-the-horse"
	profileCurrentPasswordWrong = "the current password is wrong"
	profilePasswordInvalid      = "the password must have 15 to 128 characters"
	profilePasswordChanged      = "password changed"
	profileAdminPassword        = "integration-admin-password"
	profileNoAccess             = "you don&#39;t have access"
)

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
	this.True(components.VerifyPassword(profileNewPassword, this.profilePasswordHash(userID)))

	status, header, _ := this.getPage("/profile", otherCookie)
	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))

	status, _, _ = this.getPage("/profile", cookie)
	this.Equal(http.StatusOK, status)
	this.Equal(1, this.countRows("sessions"))

	status, _, _ = this.postForm("/sign-in", "", "", url.Values{
		"identifier": {profileUserEmail},
		"password":   {profileCurrentPassword},
	})
	this.Equal(http.StatusUnauthorized, status)
	this.NotEmpty(this.signInAsUser(profileUserEmail, profileNewPassword))
}

func (this *IntegrationSuite) TestUpdateProfile_AdminGetsTheNoAccessPage() {
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage("/profile", cookie)
	this.Equal(http.StatusForbidden, status)
	this.Contains(body, profileNoAccess)
	csrfToken := this.csrfTokenFrom(body)

	status, _, body = this.postForm("/profile", cookie, csrfToken, url.Values{
		"current_password": {profileAdminPassword},
		"new_password":     {profileNewPassword},
	})
	this.Equal(http.StatusForbidden, status)
	this.Contains(body, profileNoAccess)
}
