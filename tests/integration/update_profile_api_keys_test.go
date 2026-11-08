package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const (
	updateProfileAPIKeysAnaEmail     = "ana@x.com"
	updateProfileAPIKeysBiaEmail     = "bia@x.com"
	updateProfileAPIKeysPassword     = "api-keys-correct-horse-battery"
	updateProfileAPIKeysEmptyState   = `<h2 class="h2">No API keys yet.</h2>`
	updateProfileAPIKeysNewKeyLink   = `<a class="btn sm" href="/profile/api-keys/new">New API key</a>`
	updateProfileAPIKeysNever        = "never"
	updateProfileAPIKeysExpiredBadge = `<td data-l="Expires"><span class="badge quiet">expired</span></td>`
	updateProfileAPIKeysFixedExpiry  = `<td data-l="Expires"><time class="time" datetime="2099-01-02T03:04:00Z" title="2099-01-02 03:04 UTC">2099-01-02 03:04 UTC</time></td>`
	updateProfileAPIKeysNeverUsed    = `<td data-l="Last used">never</td>`
	updateProfileAPIKeysNeverExpires = `<td data-l="Expires">never</td>`
	updateProfileAPIKeysUsedTime     = `<td data-l="Last used"><time class="time"`
	updateProfileAPIKeysCreatedTime  = `<td data-l="Created"><time class="time"`
)

var updateProfileAPIKeysFixedExpiryAt = time.Date(2099, 1, 2, 3, 4, 0, 0, time.UTC)

func (this *IntegrationSuite) updateProfileAPIKeysCreate(userID int64, name, displayPrefix string, permission model.Permission, expiresAt null.Time) (keyID int64) {
	key := &model.APIKey{
		UserID:        userID,
		KeyHash:       tokens.Hash(fmt.Sprintf("%d-%s", userID, name)),
		DisplayPrefix: displayPrefix,
		Name:          name,
		Permission:    permission,
		ExpiresAt:     expiresAt,
	}
	this.Require().True(repository.NewAPIKeyRepository(this.Pool).Create(context.Background(), key))

	return key.ID
}

func (this *IntegrationSuite) updateProfileAPIKeysRow(body, name string) string {
	start := strings.Index(body, `<td data-l="Name">`+name+`</td>`)
	this.Require().GreaterOrEqual(start, 0, "profile shows no API key named %s", name)
	end := strings.Index(body[start:], "</tr>")
	this.Require().GreaterOrEqual(end, 0)

	return body[start : start+end]
}

func (this *IntegrationSuite) TestUpdateProfileAPIKeys_NoKeysShowsTheEmptyStateAndTheNewKeyLink() {
	this.createActiveUser(updateProfileAPIKeysAnaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	cookie := this.signInAsUser(updateProfileAPIKeysAnaEmail, updateProfileAPIKeysPassword)

	status, _, body := this.getPage("/profile", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, `<h2 class="h2" id="api-keys">API keys</h2>`)
	this.Contains(body, updateProfileAPIKeysNewKeyLink)
	this.Contains(body, updateProfileAPIKeysEmptyState)
	this.NotContains(body, `<th scope="col">Last used</th>`)
}

func (this *IntegrationSuite) TestUpdateProfileAPIKeys_ListsOnlyTheUsersOwnKeysNewestFirst() {
	anaID := this.createActiveUser(updateProfileAPIKeysAnaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	biaID := this.createActiveUser(updateProfileAPIKeysBiaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	this.updateProfileAPIKeysCreate(anaID, "ana-ci", "bdk_AnaFirst", model.PermissionRead, null.Time{})
	this.updateProfileAPIKeysCreate(biaID, "bia-laptop", "bdk_BiaOnly1", model.PermissionWrite, null.Time{})
	this.updateProfileAPIKeysCreate(anaID, "ana-laptop", "bdk_AnaSecnd", model.PermissionWrite, null.Time{})
	cookie := this.signInAsUser(updateProfileAPIKeysAnaEmail, updateProfileAPIKeysPassword)

	status, _, body := this.getPage("/profile", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(body, updateProfileAPIKeysNewKeyLink)
	this.NotContains(body, updateProfileAPIKeysEmptyState)
	this.NotContains(body, "bia-laptop")
	this.NotContains(body, "bdk_BiaOnly1")
	newest := strings.Index(body, `<td data-l="Name">ana-laptop</td>`)
	oldest := strings.Index(body, `<td data-l="Name">ana-ci</td>`)
	this.GreaterOrEqual(newest, 0)
	this.Greater(oldest, newest)
	this.Contains(body, "showing 1–2 of 2")
}

func (this *IntegrationSuite) TestUpdateProfileAPIKeys_RowShowsPrefixPermissionCreationUseAndExpiry() {
	anaID := this.createActiveUser(updateProfileAPIKeysAnaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	this.updateProfileAPIKeysCreate(anaID, "fresh", "bdk_Jx3kq9ab", model.PermissionWrite, null.Time{})
	usedID := this.updateProfileAPIKeysCreate(anaID, "used", "bdk_Used0001", model.PermissionRead, null.TimeFrom(updateProfileAPIKeysFixedExpiryAt))
	repository.NewAPIKeyRepository(this.Pool).MarkUsed(context.Background(), usedID)
	this.updateProfileAPIKeysCreate(anaID, "old", "bdk_Old00001", model.PermissionRead, null.TimeFrom(time.Now().Add(-time.Minute)))
	cookie := this.signInAsUser(updateProfileAPIKeysAnaEmail, updateProfileAPIKeysPassword)

	_, _, body := this.getPage("/profile", cookie)

	fresh := this.updateProfileAPIKeysRow(body, "fresh")
	this.Contains(fresh, `<td data-l="Key"><span class="mono">bdk_Jx3kq9ab…</span></td>`)
	this.Contains(fresh, `<td data-l="Permission"><span class="badge perm">write</span></td>`)
	this.Contains(fresh, updateProfileAPIKeysCreatedTime)
	this.Contains(fresh, `just now</time></td>`)
	this.Contains(fresh, updateProfileAPIKeysNeverUsed)
	this.Contains(fresh, updateProfileAPIKeysNeverExpires)

	used := this.updateProfileAPIKeysRow(body, "used")
	this.Contains(used, `<span class="mono">bdk_Used0001…</span>`)
	this.Contains(used, `<td data-l="Permission"><span class="badge perm">read</span></td>`)
	this.Contains(used, updateProfileAPIKeysUsedTime)
	this.Contains(used, updateProfileAPIKeysFixedExpiry)
	this.NotContains(used, updateProfileAPIKeysNever)

	old := this.updateProfileAPIKeysRow(body, "old")
	this.Contains(old, updateProfileAPIKeysExpiredBadge)
	this.Contains(old, updateProfileAPIKeysNeverUsed)
}

func (this *IntegrationSuite) TestUpdateProfileAPIKeys_DemotedOwnersWriteKeyShowsItActsAsRead() {
	anaID := this.createActiveUser(updateProfileAPIKeysAnaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	this.updateProfileAPIKeysCreate(anaID, "ci-write", "bdk_Write001", model.PermissionWrite, null.Time{})
	this.updateProfileAPIKeysCreate(anaID, "ci-read", "bdk_Read0001", model.PermissionRead, null.Time{})
	_, err := this.Pool.Exec(context.Background(), `UPDATE users SET permission = 'read' WHERE id = $1`, anaID)
	this.Require().NoError(err)
	cookie := this.signInAsUser(updateProfileAPIKeysAnaEmail, updateProfileAPIKeysPassword)

	_, _, body := this.getPage("/profile", cookie)
	this.Contains(this.updateProfileAPIKeysRow(body, "ci-write"), `<td data-l="Permission"><span class="badge perm">write</span> <span class="muted">· acts as read</span></td>`)
	this.Contains(this.updateProfileAPIKeysRow(body, "ci-read"), `<td data-l="Permission"><span class="badge perm">read</span></td>`)
}

func (this *IntegrationSuite) TestUpdateProfileAPIKeys_WrongPasswordStillListsOnlyTheUsersOwnKeys() {
	anaID := this.createActiveUser(updateProfileAPIKeysAnaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	biaID := this.createActiveUser(updateProfileAPIKeysBiaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	this.updateProfileAPIKeysCreate(anaID, "ana-ci", "bdk_AnaKey01", model.PermissionWrite, null.Time{})
	this.updateProfileAPIKeysCreate(biaID, "bia-ci", "bdk_BiaKey01", model.PermissionWrite, null.Time{})
	cookie := this.signInAsUser(updateProfileAPIKeysAnaEmail, updateProfileAPIKeysPassword)
	_, _, page := this.getPage("/profile", cookie)

	status, _, body := this.postForm("/profile", cookie, this.csrfTokenFrom(page), url.Values{
		"current_password": {"not-the-current-password"},
		"new_password":     {"api-keys-new-staple-horse"},
	})
	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, `<td data-l="Name">ana-ci</td>`)
	this.Contains(body, "bdk_AnaKey01…")
	this.NotContains(body, "bia-ci")
	this.NotContains(body, "bdk_BiaKey01")
}

func (this *IntegrationSuite) TestUpdateProfileAPIKeys_PagesTwentyFiveKeysAtATime() {
	anaID := this.createActiveUser(updateProfileAPIKeysAnaEmail, model.PermissionWrite, updateProfileAPIKeysPassword)
	for index := 1; index <= 26; index++ {
		this.updateProfileAPIKeysCreate(anaID, fmt.Sprintf("key-%02d", index), fmt.Sprintf("bdk_Key000%02d", index), model.PermissionRead, null.Time{})
	}
	cookie := this.signInAsUser(updateProfileAPIKeysAnaEmail, updateProfileAPIKeysPassword)

	_, _, first := this.getPage("/profile", cookie)
	this.Contains(first, `<td data-l="Name">key-26</td>`)
	this.Contains(first, `<td data-l="Name">key-02</td>`)
	this.NotContains(first, `<td data-l="Name">key-01</td>`)
	this.Contains(first, "showing 1–25 of 26")
	this.Contains(first, `<a href="/profile?page=2" rel="next">Next</a>`)

	status, _, second := this.getPage("/profile?page=2", cookie)
	this.Equal(http.StatusOK, status)
	this.Contains(second, `<td data-l="Name">key-01</td>`)
	this.NotContains(second, `<td data-l="Name">key-02</td>`)
	this.Contains(second, "showing 26–26 of 26")

	status, _, _ = this.getPage("/profile?page=3", cookie)
	this.Equal(http.StatusNotFound, status)
	status, _, _ = this.getPage("/profile?page=0", cookie)
	this.Equal(http.StatusNotFound, status)
}
