package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	listUsersPassword   = "list-users-correct-horse"
	listUsersAdminEmail = "admin@example.com"
)

func (this *IntegrationSuite) TestListUsers_AdminSeesEveryUserByteWiseWithPermissionAndState() {
	this.createActiveUser("bob@x.com", model.PermissionRead, listUsersPassword)
	invitedID := this.createActiveUser("ana@x.com", model.PermissionWrite, listUsersPassword)
	this.setUserState(invitedID, model.UserInvited)
	deactivatedID := this.createActiveUser("Zoe@x.com", model.PermissionRead, listUsersPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, header, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, `href="/users/invite"`)

	zoe := strings.Index(body, "<td>Zoe@x.com</td>")
	ana := strings.Index(body, "<td>ana@x.com</td>")
	bob := strings.Index(body, "<td>bob@x.com</td>")
	this.Require().NotEqual(-1, zoe, "Zoe@x.com is not listed")
	this.Require().NotEqual(-1, ana, "ana@x.com is not listed")
	this.Require().NotEqual(-1, bob, "bob@x.com is not listed")
	this.Less(zoe, ana, "Zoe@x.com sorts before ana@x.com byte-wise")
	this.Less(ana, bob, "ana@x.com sorts before bob@x.com")

	this.Contains(body, "<td>Zoe@x.com</td>\n      <td>read</td>\n      <td>deactivated</td>")
	this.Contains(body, "<td>ana@x.com</td>\n      <td>write</td>\n      <td>invited</td>")
	this.Contains(body, "<td>bob@x.com</td>\n      <td>read</td>\n      <td>active</td>")
}

func (this *IntegrationSuite) TestListUsers_HtmxPageAnswersWithoutHXPushUrlAndIsNotStored() {
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("HX-Request", "true")

	status, header, _ := this.sendPageRequest(req, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Empty(header.Get("HX-Push-Url"))
	this.Equal("no-store", header.Get("Cache-Control"))
}

func (this *IntegrationSuite) TestListUsers_ShowsTheAdminRowAsAdminWithoutActions() {
	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "<td>"+listUsersAdminEmail+"</td>\n      <td>admin</td>\n      <td>active</td>")
	this.NotContains(body, fmt.Sprintf("/users/%d/", this.adminID))
}
