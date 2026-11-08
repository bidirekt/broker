package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const (
	listUserAPIKeysAnaEmail      = "ana@x.com"
	listUserAPIKeysBiaEmail      = "bia@x.com"
	listUserAPIKeysPassword      = "list-user-api-keys-correct-horse"
	listUserAPIKeysNoAccess      = "you don&#39;t have access"
	listUserAPIKeysUserNotFound  = "<h1>user not found</h1>"
	listUserAPIKeysPageNotFound  = "<h1>Not Found</h1>"
	listUserAPIKeysEmptyState    = `<h2 class="h2">No API keys yet.</h2>`
	listUserAPIKeysNewKeyPath    = "/profile/api-keys/new"
	listUserAPIKeysPasswordField = `name="current_password"`
)

func listUserAPIKeysPath(userID int64) string {
	return fmt.Sprintf("/users/%d/api-keys", userID)
}

func (this *IntegrationSuite) listUserAPIKeysCreate(userID int64, name, displayPrefix string, permission model.Permission) (keyID int64) {
	key := &model.APIKey{
		UserID:        userID,
		KeyHash:       tokens.Hash(fmt.Sprintf("list-user-api-keys-%d-%s", userID, name)),
		DisplayPrefix: displayPrefix,
		Name:          name,
		Permission:    permission,
		ExpiresAt:     null.Time{},
	}
	this.Require().True(repository.NewAPIKeyRepository(this.Pool).Create(context.Background(), key))

	return key.ID
}

func (this *IntegrationSuite) listUserAPIKeysRow(body, name string) string {
	start := strings.Index(body, `<td data-l="Name">`+name+`</td>`)
	this.Require().GreaterOrEqual(start, 0, "page shows no API key named %s", name)
	end := strings.Index(body[start:], "</tr>")
	this.Require().GreaterOrEqual(end, 0)

	return body[start : start+end]
}

func (this *IntegrationSuite) TestListUserAPIKeys_AdminSeesOnlyThatUsersKeysWithRevokeAndNoCreateForm() {
	anaID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	biaID := this.createActiveUser(listUserAPIKeysBiaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	ciID := this.listUserAPIKeysCreate(anaID, "ana-ci", "bdk_AnaFirst", model.PermissionRead)
	this.listUserAPIKeysCreate(biaID, "bia-laptop", "bdk_BiaOnly1", model.PermissionWrite)
	laptopID := this.listUserAPIKeysCreate(anaID, "ana-laptop", "bdk_AnaSecnd", model.PermissionWrite)

	status, _, body := this.getPage(listUserAPIKeysPath(anaID), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, "<h1>API keys of "+listUserAPIKeysAnaEmail+"</h1>")
	this.NotContains(body, "bia-laptop")
	this.NotContains(body, "bdk_BiaOnly1")
	newest := strings.Index(body, `<td data-l="Name">ana-laptop</td>`)
	oldest := strings.Index(body, `<td data-l="Name">ana-ci</td>`)
	this.GreaterOrEqual(newest, 0)
	this.Greater(oldest, newest)
	this.Contains(this.listUserAPIKeysRow(body, "ana-ci"), `<td data-l="Key"><span class="mono">bdk_AnaFirst…</span></td>`)
	this.Contains(this.listUserAPIKeysRow(body, "ana-ci"), `<td data-l="Last used">never</td>`)
	this.Contains(this.listUserAPIKeysRow(body, "ana-ci"), `<td data-l="Expires">never</td>`)
	for _, keyID := range []int64{ciID, laptopID} {
		this.Contains(body, fmt.Sprintf(`<td class="acts"><form method="post" action="/users/%d/api-keys/%d/revoke"><button type="submit" class="btn sm danger">Revoke</button></form></td>`, anaID, keyID))
	}
	this.NotContains(body, listUserAPIKeysNewKeyPath)
	this.NotContains(body, listUserAPIKeysPasswordField)
	this.Contains(body, "showing 1–2 of 2")
}

func (this *IntegrationSuite) TestListUserAPIKeys_PermissionIsTheEffectiveOneAgainstThatUsersPermission() {
	anaID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	this.listUserAPIKeysCreate(anaID, "ci-write", "bdk_Write001", model.PermissionWrite)
	this.listUserAPIKeysCreate(anaID, "ci-read", "bdk_Read0001", model.PermissionRead)
	_, err := this.Pool.Exec(context.Background(), `UPDATE users SET permission = 'read' WHERE id = $1`, anaID)
	this.Require().NoError(err)

	_, _, body := this.getPage(listUserAPIKeysPath(anaID), this.signInAsAdmin())

	this.Contains(this.listUserAPIKeysRow(body, "ci-write"), `<td data-l="Permission"><span class="badge perm">write</span> <span class="muted">· acts as read</span></td>`)
	this.Contains(this.listUserAPIKeysRow(body, "ci-read"), `<td data-l="Permission"><span class="badge perm">read</span></td>`)
}

func (this *IntegrationSuite) TestListUserAPIKeys_UserWithoutKeysShowsTheEmptyState() {
	anaID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)

	status, _, body := this.getPage(listUserAPIKeysPath(anaID), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	this.Contains(body, listUserAPIKeysEmptyState)
	this.NotContains(body, `<th scope="col">Last used</th>`)
}

func (this *IntegrationSuite) TestListUserAPIKeys_PagesTwentyFiveKeysAtATime() {
	anaID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	for index := 1; index <= 26; index++ {
		this.listUserAPIKeysCreate(anaID, fmt.Sprintf("key-%02d", index), fmt.Sprintf("bdk_Key000%02d", index), model.PermissionRead)
	}
	cookie := this.signInAsAdmin()

	_, _, first := this.getPage(listUserAPIKeysPath(anaID), cookie)
	this.Contains(first, `<td data-l="Name">key-26</td>`)
	this.NotContains(first, `<td data-l="Name">key-01</td>`)
	this.Contains(first, "showing 1–25 of 26")
	this.Contains(first, fmt.Sprintf(`<a href="/users/%d/api-keys?page=2" rel="next">Next</a>`, anaID))

	status, _, second := this.getPage(listUserAPIKeysPath(anaID)+"?page=2", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(second, `<td data-l="Name">key-01</td>`)
	this.Contains(second, "showing 26–26 of 26")

	for _, page := range []string{"3", "0"} {
		status, _, body := this.getPage(listUserAPIKeysPath(anaID)+"?page="+page, cookie)
		this.Equal(http.StatusNotFound, status, page)
		this.Contains(body, listUserAPIKeysPageNotFound, page)
	}
}

func (this *IntegrationSuite) TestListUserAPIKeys_NonAdminIsRefused() {
	anaID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	this.createActiveUser(listUserAPIKeysBiaEmail, model.PermissionRead, listUserAPIKeysPassword)
	this.listUserAPIKeysCreate(anaID, "ana-ci", "bdk_AnaFirst", model.PermissionRead)

	for _, email := range []string{listUserAPIKeysAnaEmail, listUserAPIKeysBiaEmail} {
		status, _, body := this.getPage(listUserAPIKeysPath(anaID), this.signInAsUser(email, listUserAPIKeysPassword))

		this.Equal(http.StatusForbidden, status, email)
		this.Contains(body, listUserAPIKeysNoAccess, email)
		this.NotContains(body, "ana-ci", email)
	}
}

func (this *IntegrationSuite) TestListUserAPIKeys_UnknownOrNonNumericUserAnswersUserNotFound() {
	anaID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	cookie := this.signInAsAdmin()

	for _, id := range []string{fmt.Sprint(anaID + 1), "abc", "0", "+" + fmt.Sprint(anaID)} {
		status, _, body := this.getPage("/users/"+id+"/api-keys", cookie)

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, listUserAPIKeysUserNotFound, id)
	}
}

func (this *IntegrationSuite) TestListUserAPIKeys_TheAdminsOwnIDIsRefused() {
	this.listUserAPIKeysCreate(this.adminID, "admin-ci", "bdk_Admin001", model.PermissionWrite)

	status, _, body := this.getPage(listUserAPIKeysPath(this.adminID), this.signInAsAdmin())

	this.Equal(http.StatusForbidden, status)
	this.Contains(body, listUserAPIKeysNoAccess)
	this.NotContains(body, "admin-ci")
}

func (this *IntegrationSuite) TestListUserAPIKeys_UsersPageLinksEveryRowButTheAdminsToItsKeys() {
	activeID := this.createActiveUser(listUserAPIKeysAnaEmail, model.PermissionWrite, listUserAPIKeysPassword)
	deactivatedID := this.createActiveUser(listUserAPIKeysBiaEmail, model.PermissionRead, listUserAPIKeysPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	status, _, body := this.getPage("/users", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status)
	for _, userID := range []int64{activeID, deactivatedID} {
		this.Contains(body, fmt.Sprintf(`<a class="btn sm" href="/users/%d/api-keys">API keys</a>`, userID))
	}
	this.NotContains(body, listUserAPIKeysPath(this.adminID))
	this.Equal(1, strings.Count(body, `>You<`))
}
