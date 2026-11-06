package integration_test

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	inviteUserEmail             = "ana@x.com"
	inviteUserEmailInvalid      = "invalid email"
	inviteUserPermissionInvalid = "invalid permission"
	inviteUserEmailInUse        = "email already in use"
	inviteUserReadDescription   = "Sees every data screen and their own profile; runs can-i-deploy and validate."
	inviteUserWriteDescription  = "Everything read does, plus publishing contracts, recording deployments, and creating and renaming participants and environments."
	inviteUserEmailError        = `aria-invalid="true" aria-describedby="email-err">`
	inviteUserPermissionError   = `role="radiogroup" aria-labelledby="permission-label" aria-invalid="true" aria-describedby="permission-err">`
)

func inviteUserFieldError(field string) string {
	return `<p class="err" id="` + field + `-err"><svg`
}

func inviteUserFieldMessage(message string) string {
	return `</svg><span>` + message + `</span></p>`
}

var inviteUserLinkInPage = regexp.MustCompile(`http://localhost:8080/set-password\?token=([A-Za-z0-9_-]{43})`)

func (this *IntegrationSuite) inviteUser(email, permission string) (status int, body string) {
	cookie := this.signInAsAdmin()
	_, _, form := this.getPage("/users/invite", cookie)

	status, _, body = this.postForm("/users/invite", cookie, this.csrfTokenFrom(form), url.Values{
		"email":      {email},
		"permission": {permission},
	})

	return status, body
}

func (this *IntegrationSuite) TestInviteUser_FormAsksForEmailAndPermissionWithReadChecked() {
	status, _, body := this.getPage("/users/invite", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, `<form class="sheet formcard" method="post" action="/users/invite">`)
	this.Contains(body, `name="email" type="text" inputmode="email" autocomplete="off" value="" required autofocus>`)
	this.Contains(body, `<input type="radio" name="permission" value="read" required checked>`)
	this.Contains(body, `<input type="radio" name="permission" value="write" required>`)
	this.Contains(body, inviteUserReadDescription)
	this.Contains(body, inviteUserWriteDescription)
	this.Contains(body, `<button type="submit" class="btn primary">Invite</button>`)
	this.NotContains(body, `aria-invalid`)
}

func (this *IntegrationSuite) TestInviteUser_HtmxFormAnswersWithoutHXPushUrl() {
	req := httptest.NewRequest("GET", "/users/invite", nil)
	req.Header.Set("HX-Request", "true")

	status, header, body := this.sendPageRequest(req, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, `method="post" action="/users/invite">`)
	this.Empty(header.Get("HX-Push-Url"))
}

func (this *IntegrationSuite) TestInviteUser_CreatesAnInvitedUserAndShowsTheLinkOnce() {
	status, body := this.inviteUser(inviteUserEmail, "read")

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "<h1>User invited</h1>")
	match := inviteUserLinkInPage.FindStringSubmatch(body)
	this.Require().Len(match, 2, "page shows no invite link")
	this.Contains(body, `<input class="copy" type="text" readonly value="`+match[0]+`" aria-label="Invite link">`)

	tokenHash := sha256.Sum256([]byte(match[1]))
	var purpose, email string
	var expiresAt time.Time
	err := this.Pool.QueryRow(context.Background(),
		`SELECT one_time_links.purpose, one_time_links.expires_at, users.email
		FROM one_time_links JOIN users ON users.id = one_time_links.user_id
		WHERE one_time_links.token_hash = $1 AND one_time_links.used_at IS NULL`,
		tokenHash[:],
	).Scan(&purpose, &expiresAt, &email)
	this.Require().NoError(err)
	this.Equal("invite", purpose)
	this.Equal(inviteUserEmail, email)
	this.WithinDuration(time.Now().Add(24*time.Hour), expiresAt, time.Minute)
	this.Contains(body, `Send this invite link to `+inviteUserEmail+`. It works once, until <time class="time" datetime="`+expiresAt.UTC().Format(time.RFC3339Nano)+`" title="`+expiresAt.UTC().Format("2006-01-02 15:04")+` UTC">`+expiresAt.UTC().Format("2006-01-02 15:04")+` UTC</time>, and is not shown again.`)
	this.Equal(1, this.countRows("one_time_links"))

	_, _, list := this.getPage("/users", this.signInAsAdmin())
	this.Contains(list, "<tr data-state=\"invited\">\n        <td data-l=\"Email\"><span class=\"mono\">ana@x.com</span></td>\n        <td data-l=\"Permission\"><span class=\"badge perm\">read</span></td>")
	this.NotContains(list, match[1])
}

func (this *IntegrationSuite) TestInviteUser_HtmxInviteShowsTheLinkWithoutPushingTheFormURLOrBeingStored() {
	cookie := this.signInAsAdmin()
	_, _, form := this.getPage("/users/invite", cookie)

	req := httptest.NewRequest("POST", "/users/invite", strings.NewReader(url.Values{
		"email":      {inviteUserEmail},
		"permission": {"read"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("X-CSRF-Token", this.csrfTokenFrom(form))
	req.Header.Set("HX-Request", "true")

	status, header, body := this.sendPageRequest(req, cookie)

	this.Require().Equal(http.StatusOK, status)
	this.Regexp(inviteUserLinkInPage, body)
	this.Equal("false", header.Get("HX-Push-Url"))
	this.Equal("no-store", header.Get("Cache-Control"))
}

func (this *IntegrationSuite) TestInviteUser_RejectsAnEmailInUseInAnotherCase() {
	this.deleteSuiteWriter()

	status, _ := this.inviteUser(inviteUserEmail, "write")
	this.Require().Equal(http.StatusOK, status)

	status, body := this.inviteUser("ANA@X.com", "read")

	this.Equal(http.StatusConflict, status)
	this.Contains(body, inviteUserEmailError)
	this.Contains(body, inviteUserFieldError("email"))
	this.Contains(body, inviteUserFieldMessage(inviteUserEmailInUse))
	this.Contains(body, `value="ANA@X.com"`)
	this.Contains(body, `value="read" required checked>`)
	this.Equal(2, this.countRows("users"))
	this.Equal(1, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestInviteUser_RejectsAMalformedEmail() {
	this.deleteSuiteWriter()

	for _, email := range []string{"ana.x.com", "ana @x.com"} {
		status, body := this.inviteUser(email, "read")

		this.Equal(http.StatusBadRequest, status, email)
		this.Contains(body, inviteUserEmailError, email)
		this.Contains(body, inviteUserFieldMessage(inviteUserEmailInvalid), email)
		this.NotContains(body, `aria-describedby="permission-err"`, email)
	}

	this.Equal(1, this.countRows("users"))
}

func (this *IntegrationSuite) TestInviteUser_RejectsATamperedPermission() {
	this.deleteSuiteWriter()

	for _, permission := range []string{"admin", "Write", ""} {
		status, body := this.inviteUser(inviteUserEmail, permission)

		this.Equal(http.StatusBadRequest, status, permission)
		this.Contains(body, inviteUserPermissionError, permission)
		this.Contains(body, inviteUserFieldError("permission"), permission)
		this.Contains(body, inviteUserFieldMessage(inviteUserPermissionInvalid), permission)
		this.NotContains(body, " checked", permission)
		this.Contains(body, `<input type="radio" name="permission" value="read" required autofocus>`, permission)
		this.Contains(body, `value="`+inviteUserEmail+`" required>`, permission)
	}

	this.Equal(1, this.countRows("users"))
}

func (this *IntegrationSuite) TestInviteUser_ChecksTheEmailBeforeThePermission() {
	status, body := this.inviteUser("ana.x.com", "admin")

	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, inviteUserEmailInvalid)
}

func (this *IntegrationSuite) TestInviteUser_ChecksThePermissionBeforeTheClash() {
	status, _ := this.inviteUser(inviteUserEmail, "write")
	this.Require().Equal(http.StatusOK, status)

	status, body := this.inviteUser(inviteUserEmail, "admin")

	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, inviteUserPermissionInvalid)
}
