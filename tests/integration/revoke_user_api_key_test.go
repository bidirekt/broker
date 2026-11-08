package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const (
	revokeUserAPIKeyAnaEmail     = "ana@x.com"
	revokeUserAPIKeyBiaEmail     = "bia@x.com"
	revokeUserAPIKeyPassword     = "revoke-user-api-key-correct-horse"
	revokeUserAPIKeyName         = "ana-laptop"
	revokeUserAPIKeyNotice       = `<div class="flash" role="status">`
	revokeUserAPIKeyNoAccess     = "you don&#39;t have access"
	revokeUserAPIKeyNotFound     = "<h1>API key not found</h1>"
	revokeUserAPIKeyUserNotFound = "<h1>user not found</h1>"
)

func revokeUserAPIKeyPath(userID, keyID string) string {
	return "/users/" + userID + "/api-keys/" + keyID + "/revoke"
}

func (this *IntegrationSuite) revokeUserAPIKeyCreate(userID int64, name string) (key string, keyID int64) {
	random, err := tokens.Random()
	this.Require().NoError(err)
	key = "bdk_" + random

	stored := &model.APIKey{
		UserID:        userID,
		KeyHash:       tokens.Hash(key),
		DisplayPrefix: "bdk_" + random[:8],
		Name:          name,
		Permission:    model.PermissionRead,
		ExpiresAt:     null.Time{},
	}
	this.Require().True(repository.NewAPIKeyRepository(this.Pool).Create(context.Background(), stored))

	return key, stored.ID
}

func (this *IntegrationSuite) revokeUserAPIKeyCallAPI(key string) (status int, body string) {
	status, _, body = this.sendApiRequest(httptest.NewRequest(http.MethodGet, "/api/participants", nil), key)

	return status, body
}

func (this *IntegrationSuite) revokeUserAPIKeyPost(cookie, userID, keyID string) (status int, header http.Header, body string) {
	_, _, profile := this.getPage("/profile", cookie)

	return this.postForm(revokeUserAPIKeyPath(userID, keyID), cookie, this.csrfTokenFrom(profile), url.Values{})
}

func (this *IntegrationSuite) revokeUserAPIKeyExists(keyID int64) bool {
	var exists bool
	err := this.Pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM api_keys WHERE id = $1)`, keyID).Scan(&exists)
	this.Require().NoError(err)

	return exists
}

func (this *IntegrationSuite) TestRevokeUserAPIKey_AdminRevokesTheKeyAndItsNextAPIRequestIsUnauthenticated() {
	anaID := this.createActiveUser(revokeUserAPIKeyAnaEmail, model.PermissionWrite, revokeUserAPIKeyPassword)
	key, keyID := this.revokeUserAPIKeyCreate(anaID, revokeUserAPIKeyName)
	cookie := this.signInAsAdmin()

	status, body := this.revokeUserAPIKeyCallAPI(key)
	this.Require().Equal(http.StatusOK, status, body)

	status, header, _ := this.revokeUserAPIKeyPost(cookie, fmt.Sprint(anaID), fmt.Sprint(keyID))
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal(fmt.Sprintf("/users/%d/api-keys", anaID), header.Get("Location"))

	_, _, page := this.getPage(header.Get("Location"), cookie)
	this.Contains(page, revokeUserAPIKeyNotice)
	this.Contains(page, "<span>API key "+revokeUserAPIKeyName+" revoked</span>")
	this.NotContains(page, `<td data-l="Name">`+revokeUserAPIKeyName+`</td>`)
	this.Contains(page, "No API keys yet.")
	this.False(this.revokeUserAPIKeyExists(keyID))

	status, body = this.revokeUserAPIKeyCallAPI(key)
	this.Equal(http.StatusUnauthorized, status)
	this.JSONEq(`{"message":"unauthenticated"}`, body)
}

func (this *IntegrationSuite) TestRevokeUserAPIKey_AKeyUnderAnotherUsersIDAnswersNotFoundAndSurvives() {
	anaID := this.createActiveUser(revokeUserAPIKeyAnaEmail, model.PermissionWrite, revokeUserAPIKeyPassword)
	biaID := this.createActiveUser(revokeUserAPIKeyBiaEmail, model.PermissionWrite, revokeUserAPIKeyPassword)
	key, keyID := this.revokeUserAPIKeyCreate(anaID, revokeUserAPIKeyName)
	cookie := this.signInAsAdmin()

	for _, id := range []string{fmt.Sprint(keyID + 1), "abc", "0", "+" + fmt.Sprint(keyID)} {
		status, _, body := this.revokeUserAPIKeyPost(cookie, fmt.Sprint(anaID), id)

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, revokeUserAPIKeyNotFound, id)
	}

	status, _, body := this.revokeUserAPIKeyPost(cookie, fmt.Sprint(biaID), fmt.Sprint(keyID))

	this.Equal(http.StatusNotFound, status)
	this.Contains(body, revokeUserAPIKeyNotFound)
	this.True(this.revokeUserAPIKeyExists(keyID))
	status, body = this.revokeUserAPIKeyCallAPI(key)
	this.Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) TestRevokeUserAPIKey_UnknownOrNonNumericUserAnswersUserNotFound() {
	anaID := this.createActiveUser(revokeUserAPIKeyAnaEmail, model.PermissionWrite, revokeUserAPIKeyPassword)
	_, keyID := this.revokeUserAPIKeyCreate(anaID, revokeUserAPIKeyName)
	cookie := this.signInAsAdmin()

	for _, id := range []string{fmt.Sprint(anaID + 1), "abc", "0", "+" + fmt.Sprint(anaID)} {
		status, _, body := this.revokeUserAPIKeyPost(cookie, id, fmt.Sprint(keyID))

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, revokeUserAPIKeyUserNotFound, id)
	}

	this.True(this.revokeUserAPIKeyExists(keyID))
}

func (this *IntegrationSuite) TestRevokeUserAPIKey_NonAdminIsRefusedAndTheKeySurvives() {
	anaID := this.createActiveUser(revokeUserAPIKeyAnaEmail, model.PermissionWrite, revokeUserAPIKeyPassword)
	this.createActiveUser(revokeUserAPIKeyBiaEmail, model.PermissionWrite, revokeUserAPIKeyPassword)
	_, keyID := this.revokeUserAPIKeyCreate(anaID, revokeUserAPIKeyName)

	for _, email := range []string{revokeUserAPIKeyAnaEmail, revokeUserAPIKeyBiaEmail} {
		cookie := this.signInAsUser(email, revokeUserAPIKeyPassword)
		status, _, body := this.revokeUserAPIKeyPost(cookie, fmt.Sprint(anaID), fmt.Sprint(keyID))

		this.Equal(http.StatusForbidden, status, email)
		this.Contains(body, revokeUserAPIKeyNoAccess, email)
	}

	this.True(this.revokeUserAPIKeyExists(keyID))
}

func (this *IntegrationSuite) TestRevokeUserAPIKey_TheAdminsOwnIDIsRefusedAndTheKeySurvives() {
	key, keyID := this.revokeUserAPIKeyCreate(this.adminID, "admin-ci")

	status, _, body := this.revokeUserAPIKeyPost(this.signInAsAdmin(), fmt.Sprint(this.adminID), fmt.Sprint(keyID))

	this.Equal(http.StatusForbidden, status)
	this.Contains(body, revokeUserAPIKeyNoAccess)
	this.True(this.revokeUserAPIKeyExists(keyID))
	status, body = this.revokeUserAPIKeyCallAPI(key)
	this.Equal(http.StatusOK, status, body)
}
