package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	listUsersPassword   = "list-users-correct-horse"
	listUsersAdminEmail = "admin@example.com"
	listUsersSubtitle   = `<p class="sub">Everyone with an account on this broker.</p>`
)

var listUsersRowInPage = regexp.MustCompile(`<td data-l="Email"><span class="mono">([^<]+)</span></td>`)

func listUsersRow(email string, permission model.Permission, state model.UserState) string {
	return fmt.Sprintf("<tr data-state=\"%s\">\n        <td data-l=\"Email\"><span class=\"mono\">%s</span></td>\n        <td data-l=\"Permission\"><span class=\"badge perm\">%s</span></td>", state, email, permission)
}

func (this *IntegrationSuite) listUsersCreateInvited(count int) {
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO users (email, permission) SELECT 'user' || lpad(i::text, 2, '0') || '@x.com', 'read' FROM generate_series(1, $1) i`,
		count,
	)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) TestListUsers_AdminSeesEveryUserByteWiseWithPermissionAndState() {
	this.createActiveUser("bob@x.com", model.PermissionRead, listUsersPassword)
	invitedID := this.createActiveUser("ana@x.com", model.PermissionWrite, listUsersPassword)
	this.setUserState(invitedID, model.UserInvited)
	deactivatedID := this.createActiveUser("Zoe@x.com", model.PermissionRead, listUsersPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, header, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<a class="btn primary" href="/users/invite">Invite a user</a>`)
	this.Contains(body, listUsersSubtitle)

	zoe := strings.Index(body, `<span class="mono">Zoe@x.com</span>`)
	ana := strings.Index(body, `<span class="mono">ana@x.com</span>`)
	bob := strings.Index(body, `<span class="mono">bob@x.com</span>`)
	this.Require().NotEqual(-1, zoe, "Zoe@x.com is not listed")
	this.Require().NotEqual(-1, ana, "ana@x.com is not listed")
	this.Require().NotEqual(-1, bob, "bob@x.com is not listed")
	this.Less(zoe, ana, "Zoe@x.com sorts before ana@x.com byte-wise")
	this.Less(ana, bob, "ana@x.com sorts before bob@x.com")

	this.Contains(body, listUsersRow("Zoe@x.com", model.PermissionRead, model.UserDeactivated))
	this.Contains(body, listUsersRow("ana@x.com", model.PermissionWrite, model.UserInvited))
	this.Contains(body, listUsersRow("bob@x.com", model.PermissionRead, model.UserActive))
	this.Contains(body, `</svg>deactivated</span>`)
	this.Contains(body, `</svg>invited</span>`)
	this.Contains(body, `<span class="badge yes">`)
}

func (this *IntegrationSuite) TestListUsers_HtmxPageAnswersWithoutHXPushUrlAndIsNotStored() {
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("HX-Request", "true")

	status, header, _ := this.sendPageRequest(req, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Empty(header.Get("HX-Push-Url"))
	this.Equal("no-store", header.Get("Cache-Control"))
}

func (this *IntegrationSuite) TestListUsers_ShowsTheAdminRowAsYouWithoutActions() {
	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, listUsersRow(listUsersAdminEmail, model.PermissionAdmin, model.UserActive))
	this.Contains(body, `<span class="muted">You</span>`)
	this.Equal(1, strings.Count(body, `>You<`))
	this.NotContains(body, fmt.Sprintf("/users/%d/", this.adminID))
}

func (this *IntegrationSuite) TestListUsers_PagesTwentyFiveUsersAtATime() {
	this.listUsersCreateInvited(30)
	cookie := this.signInAsAdmin()

	status, _, first := this.getPage("/users", cookie)

	this.Require().Equal(http.StatusOK, status)
	this.Len(listUsersRowInPage.FindAllString(first, -1), 25)
	this.Contains(first, "<span>showing 1–25 of 32</span>")
	this.Contains(first, `<span class="off">Previous</span>`)
	this.Contains(first, `<span aria-current="page">1</span>`)
	this.Contains(first, `<a href="/users?page=2" rel="next">Next</a>`)

	status, _, second := this.getPage("/users?page=2", cookie)

	this.Require().Equal(http.StatusOK, status)
	this.Equal([]string{"user24@x.com", "user25@x.com", "user26@x.com", "user27@x.com", "user28@x.com", "user29@x.com", "user30@x.com"}, listUsersEmailsIn(second))
	this.Contains(second, "<span>showing 26–32 of 32</span>")
	this.Contains(second, `<a href="/users?page=1" rel="prev">Previous</a>`)
	this.Contains(second, `<span class="off">Next</span>`)
}

func listUsersEmailsIn(body string) (emails []string) {
	for _, match := range listUsersRowInPage.FindAllStringSubmatch(body, -1) {
		emails = append(emails, match[1])
	}

	return emails
}

func (this *IntegrationSuite) TestListUsers_OnePageIsShownWithBothDirectionsOff() {
	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "<span>showing 1–2 of 2</span>")
	this.Contains(body, `<span class="off">Previous</span>`)
	this.Contains(body, `<span class="off">Next</span>`)
}

func (this *IntegrationSuite) TestListUsers_AnswersNotFoundForAPagePastTheLastOrNotCanonical() {
	this.listUsersCreateInvited(30)
	cookie := this.signInAsAdmin()

	for _, page := range []string{"3", "0", "01", "%2B1", "", "abc", "-1"} {
		status, _, body := this.getPage("/users?page="+page, cookie)

		this.Equal(http.StatusNotFound, status, page)
		this.Contains(body, "<h1>Not Found</h1>", page)
	}
}

func (this *IntegrationSuite) TestListUsers_AnswersNotFoundForAPageWhoseOffsetWouldOverflow() {
	status, _, body := this.getPage("/users?page=400000000000000000", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
	this.Contains(body, "<h1>Not Found</h1>")
}

func (this *IntegrationSuite) TestListUsers_RowActionsCarryThePageTheyCameFrom() {
	this.listUsersCreateInvited(30)
	activeID := this.createActiveUser("zz-active@x.com", model.PermissionRead, listUsersPassword)
	deactivatedID := this.createActiveUser("zz-deactivated@x.com", model.PermissionRead, listUsersPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, _, body := this.getPage("/users?page=2", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, fmt.Sprintf(`<a class="btn sm" href="/users/%d/edit?page=2">Edit</a>`, activeID))
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/deactivate"><input type="hidden" name="page" value="2">`, activeID))
	this.Contains(body, fmt.Sprintf(`<form method="post" action="/users/%d/reactivate"><input type="hidden" name="page" value="2">`, deactivatedID))
}
