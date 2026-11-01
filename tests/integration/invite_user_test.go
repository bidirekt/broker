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
)

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

func (this *IntegrationSuite) TestInviteUser_FormAsksForEmailAndPermission() {
	status, _, body := this.getPage("/users/invite", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, `<form method="post" action="/users/invite">`)
	this.Contains(body, `name="email"`)
	this.Contains(body, `<option value="read"`)
	this.Contains(body, `<option value="write"`)
}

func (this *IntegrationSuite) TestInviteUser_HtmxFormAnswersWithoutHXPushUrl() {
	req := httptest.NewRequest("GET", "/users/invite", nil)
	req.Header.Set("HX-Request", "true")

	status, header, body := this.sendPageRequest(req, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, `<form method="post" action="/users/invite">`)
	this.Empty(header.Get("HX-Push-Url"))
}

func (this *IntegrationSuite) TestInviteUser_CreatesAnInvitedUserAndShowsTheLinkOnce() {
	status, body := this.inviteUser(inviteUserEmail, "read")

	this.Require().Equal(http.StatusOK, status)
	match := inviteUserLinkInPage.FindStringSubmatch(body)
	this.Require().Len(match, 2, "page shows no invite link")

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
	this.Equal(1, this.countRows("one_time_links"))

	_, _, list := this.getPage("/users", this.signInAsAdmin())
	this.Contains(list, "<td>ana@x.com</td>\n      <td>read</td>\n      <td>invited</td>")
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
	status, _ := this.inviteUser(inviteUserEmail, "write")
	this.Require().Equal(http.StatusOK, status)

	status, body := this.inviteUser("ANA@X.com", "read")

	this.Equal(http.StatusConflict, status)
	this.Contains(body, inviteUserEmailInUse)
	this.Equal(1, this.countRows("users"))
	this.Equal(1, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestInviteUser_RejectsAMalformedEmail() {
	for _, email := range []string{"ana.x.com", "ana @x.com"} {
		status, body := this.inviteUser(email, "read")

		this.Equal(http.StatusBadRequest, status, email)
		this.Contains(body, inviteUserEmailInvalid, email)
	}

	this.Equal(0, this.countRows("users"))
}

func (this *IntegrationSuite) TestInviteUser_RejectsATamperedPermission() {
	for _, permission := range []string{"admin", "Write", ""} {
		status, body := this.inviteUser(inviteUserEmail, permission)

		this.Equal(http.StatusBadRequest, status, permission)
		this.Contains(body, inviteUserPermissionInvalid, permission)
	}

	this.Equal(0, this.countRows("users"))
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
