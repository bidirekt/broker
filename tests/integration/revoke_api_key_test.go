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
	revokeAPIKeyEmail         = "ana@x.com"
	revokeAPIKeyPassword      = "revoke-api-key-correct-horse"
	revokeAPIKeyOtherEmail    = "bob@x.com"
	revokeAPIKeyOtherPassword = "revoke-api-key-other-horse"
	revokeAPIKeyName          = "ana-laptop"
	revokeAPIKeyNotice        = `<div class="flash" role="status">`
	revokeAPIKeyNotFoundPage  = "<h1>API key not found</h1>"
	revokeAPIKeyErrorStatus   = `<div class="code-n">404</div>`
)

func revokeAPIKeyPath(keyID string) string {
	return "/profile/api-keys/" + keyID + "/revoke"
}

func (this *IntegrationSuite) revokeAPIKeyCreate(userID int64, name string) (key string, keyID int64) {
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

func (this *IntegrationSuite) revokeAPIKeyCallAPI(key string) (status int, body string) {
	status, _, body = this.sendApiRequest(httptest.NewRequest(http.MethodGet, "/api/participants", nil), key)

	return status, body
}

func (this *IntegrationSuite) revokeAPIKeyPost(cookie, keyID string) (status int, header http.Header, body string) {
	_, _, profile := this.getPage("/profile", cookie)

	return this.postForm(revokeAPIKeyPath(keyID), cookie, this.csrfTokenFrom(profile), url.Values{})
}

func (this *IntegrationSuite) revokeAPIKeyExists(keyID int64) bool {
	var exists bool
	err := this.Pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM api_keys WHERE id = $1)`, keyID).Scan(&exists)
	this.Require().NoError(err)

	return exists
}

func (this *IntegrationSuite) TestRevokeAPIKey_RemovesTheKeyFromTheListAndEndsItsAPIAccess() {
	userID := this.createActiveUser(revokeAPIKeyEmail, model.PermissionWrite, revokeAPIKeyPassword)
	key, keyID := this.revokeAPIKeyCreate(userID, revokeAPIKeyName)
	cookie := this.signInAsUser(revokeAPIKeyEmail, revokeAPIKeyPassword)

	status, body := this.revokeAPIKeyCallAPI(key)
	this.Require().Equal(http.StatusOK, status, body)

	status, header, _ := this.revokeAPIKeyPost(cookie, fmt.Sprint(keyID))
	this.Require().Equal(http.StatusSeeOther, status)
	this.Equal("/profile", header.Get("Location"))

	_, _, profile := this.getPage("/profile", cookie)
	this.Contains(profile, revokeAPIKeyNotice)
	this.Contains(profile, "<span>API key "+revokeAPIKeyName+" revoked</span>")
	this.NotContains(profile, `<td data-l="Name">`+revokeAPIKeyName+`</td>`)
	this.Contains(profile, "No API keys yet.")
	this.False(this.revokeAPIKeyExists(keyID))

	status, body = this.revokeAPIKeyCallAPI(key)
	this.Equal(http.StatusUnauthorized, status)
	this.JSONEq(`{"message":"unauthenticated"}`, body)
}

func (this *IntegrationSuite) TestRevokeAPIKey_AnotherUsersKeyAnswersNotFoundAndSurvives() {
	this.createActiveUser(revokeAPIKeyEmail, model.PermissionWrite, revokeAPIKeyPassword)
	otherID := this.createActiveUser(revokeAPIKeyOtherEmail, model.PermissionWrite, revokeAPIKeyOtherPassword)
	key, keyID := this.revokeAPIKeyCreate(otherID, revokeAPIKeyName)
	cookie := this.signInAsUser(revokeAPIKeyEmail, revokeAPIKeyPassword)

	status, _, body := this.revokeAPIKeyPost(cookie, fmt.Sprint(keyID))

	this.Equal(http.StatusNotFound, status)
	this.Contains(body, revokeAPIKeyNotFoundPage)
	this.Contains(body, revokeAPIKeyErrorStatus)
	this.True(this.revokeAPIKeyExists(keyID))
	status, body = this.revokeAPIKeyCallAPI(key)
	this.Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) TestRevokeAPIKey_AnswersNotFoundForAnUnknownOrNonNumericID() {
	userID := this.createActiveUser(revokeAPIKeyEmail, model.PermissionWrite, revokeAPIKeyPassword)
	_, keyID := this.revokeAPIKeyCreate(userID, revokeAPIKeyName)
	cookie := this.signInAsUser(revokeAPIKeyEmail, revokeAPIKeyPassword)

	for _, id := range []string{fmt.Sprint(keyID + 1), "abc", "0", "0" + fmt.Sprint(keyID), "+" + fmt.Sprint(keyID)} {
		status, _, body := this.revokeAPIKeyPost(cookie, id)

		this.Equal(http.StatusNotFound, status, id)
		this.Contains(body, revokeAPIKeyNotFoundPage, id)
		this.Contains(body, revokeAPIKeyErrorStatus, id)
	}

	this.True(this.revokeAPIKeyExists(keyID))
}

func (this *IntegrationSuite) TestRevokeAPIKey_WithoutASessionRedirectsToSignInAndKeepsTheKey() {
	userID := this.createActiveUser(revokeAPIKeyEmail, model.PermissionWrite, revokeAPIKeyPassword)
	_, keyID := this.revokeAPIKeyCreate(userID, revokeAPIKeyName)

	status, header, _ := this.postForm(revokeAPIKeyPath(fmt.Sprint(keyID)), "", "", url.Values{})

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
	this.True(this.revokeAPIKeyExists(keyID))
}

func (this *IntegrationSuite) TestRevokeAPIKey_WithoutTheCSRFTokenAnswersForbiddenAndKeepsTheKey() {
	userID := this.createActiveUser(revokeAPIKeyEmail, model.PermissionWrite, revokeAPIKeyPassword)
	_, keyID := this.revokeAPIKeyCreate(userID, revokeAPIKeyName)
	cookie := this.signInAsUser(revokeAPIKeyEmail, revokeAPIKeyPassword)

	for _, csrfToken := range []string{"", "not-the-token"} {
		status, _, _ := this.postForm(revokeAPIKeyPath(fmt.Sprint(keyID)), cookie, csrfToken, url.Values{})

		this.Equal(http.StatusForbidden, status, csrfToken)
	}

	this.True(this.revokeAPIKeyExists(keyID))
}

func (this *IntegrationSuite) TestRevokeAPIKey_ButtonIsOnEveryProfileRow() {
	userID := this.createActiveUser(revokeAPIKeyEmail, model.PermissionWrite, revokeAPIKeyPassword)
	_, laptopID := this.revokeAPIKeyCreate(userID, revokeAPIKeyName)
	_, ciID := this.revokeAPIKeyCreate(userID, "ana-ci")

	status, _, body := this.getPage("/profile", this.signInAsUser(revokeAPIKeyEmail, revokeAPIKeyPassword))

	this.Require().Equal(http.StatusOK, status)
	for _, keyID := range []int64{laptopID, ciID} {
		this.Contains(body, fmt.Sprintf(`<td class="acts"><form method="post" action="/profile/api-keys/%d/revoke"><button type="submit" class="btn sm danger">Revoke</button></form></td>`, keyID))
	}
}
