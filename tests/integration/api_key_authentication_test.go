package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const (
	apiKeyAuthenticationEmail           = "kira@x.com"
	apiKeyAuthenticationPassword        = "kira-correct-horse-battery"
	apiKeyAuthenticationUnauthenticated = `{"message":"unauthenticated"}`
	apiKeyAuthenticationForbidden       = `{"message":"forbidden"}`
	apiKeyAuthenticationParticipants    = "/api/participants"
	apiKeyAuthenticationWriteBody       = `{"participant":"billing"}`
)

func (this *IntegrationSuite) apiKeyAuthenticationCreateKey(userID int64, permission model.Permission, expiresAt null.Time) (key string, stored *model.APIKey) {
	random, err := tokens.Random()
	this.Require().NoError(err)
	key = "bdk_" + random

	stored = &model.APIKey{
		UserID:        userID,
		KeyHash:       tokens.Hash(key),
		DisplayPrefix: "bdk_" + random[:8],
		Name:          "kira-laptop",
		Permission:    permission,
		ExpiresAt:     expiresAt,
	}
	this.Require().True(repository.NewAPIKeyRepository(this.Pool).Create(context.Background(), stored))

	return key, stored
}

func (this *IntegrationSuite) apiKeyAuthenticationRead(key string) (status int, header http.Header, body string) {
	return this.sendApiRequest(httptest.NewRequest(http.MethodGet, apiKeyAuthenticationParticipants, nil), key)
}

func (this *IntegrationSuite) apiKeyAuthenticationWrite(key string) (status int, body string) {
	req := httptest.NewRequest(http.MethodPost, apiKeyAuthenticationParticipants, strings.NewReader(apiKeyAuthenticationWriteBody))
	req.Header.Set("Content-Type", "application/json")

	status, _, body = this.sendApiRequest(req, key)

	return status, body
}

func (this *IntegrationSuite) apiKeyAuthenticationAssertRefused(key string) {
	status, header, body := this.apiKeyAuthenticationRead(key)
	this.Equal(http.StatusUnauthorized, status)
	this.Equal("Bearer", header.Get("WWW-Authenticate"))
	this.JSONEq(apiKeyAuthenticationUnauthenticated, body)
}

func (this *IntegrationSuite) apiKeyAuthenticationLastUsedAt(keyID int64) (lastUsedAt null.Time) {
	err := this.Pool.QueryRow(context.Background(), `SELECT last_used_at FROM api_keys WHERE id = $1`, keyID).Scan(&lastUsedAt)
	this.Require().NoError(err)

	return lastUsedAt
}

func (this *IntegrationSuite) TestApiKeyAuthentication_ReadKeyIsAllowedToReadAndForbiddenToWrite() {
	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	key, _ := this.apiKeyAuthenticationCreateKey(userID, model.PermissionRead, null.Time{})

	status, _, body := this.apiKeyAuthenticationRead(key)
	this.Equal(http.StatusOK, status, body)

	status, body = this.apiKeyAuthenticationWrite(key)
	this.Equal(http.StatusForbidden, status)
	this.JSONEq(apiKeyAuthenticationForbidden, body)
	this.Equal(0, this.countRows("participants"))
}

func (this *IntegrationSuite) TestApiKeyAuthentication_WriteKeyOfADemotedUserIsForbiddenToWriteFromTheNextRequest() {
	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	key, _ := this.apiKeyAuthenticationCreateKey(userID, model.PermissionWrite, null.Time{})

	status, body := this.apiKeyAuthenticationWrite(key)
	this.Require().Equal(http.StatusOK, status, body)

	_, err := this.Pool.Exec(context.Background(), `UPDATE users SET permission = 'read' WHERE id = $1`, userID)
	this.Require().NoError(err)

	status, body = this.apiKeyAuthenticationWrite(key)
	this.Equal(http.StatusForbidden, status)
	this.JSONEq(apiKeyAuthenticationForbidden, body)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_RefusesAnExpiredKey() {
	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	key, _ := this.apiKeyAuthenticationCreateKey(userID, model.PermissionWrite, null.TimeFrom(time.Now().Add(-time.Second)))

	this.apiKeyAuthenticationAssertRefused(key)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_RefusesARevokedKey() {
	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	key, stored := this.apiKeyAuthenticationCreateKey(userID, model.PermissionWrite, null.TimeFrom(time.Now().Add(time.Hour)))

	status, _, body := this.apiKeyAuthenticationRead(key)
	this.Require().Equal(http.StatusOK, status, body)

	_, deleted := repository.NewAPIKeyRepository(this.Pool).DeleteByIDAndUser(context.Background(), stored.ID, userID)
	this.Require().True(deleted)

	this.apiKeyAuthenticationAssertRefused(key)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_RefusesAnUnknownKey() {
	random, err := tokens.Random()
	this.Require().NoError(err)

	this.apiKeyAuthenticationAssertRefused("bdk_" + random)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_RefusesTheKeyOfADeactivatedUser() {
	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	key, _ := this.apiKeyAuthenticationCreateKey(userID, model.PermissionWrite, null.Time{})

	this.setUserState(userID, model.UserDeactivated)

	this.apiKeyAuthenticationAssertRefused(key)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_TheSignInTokenStillWorks() {
	this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	token := this.signInToApi(apiKeyAuthenticationEmail, apiKeyAuthenticationPassword)

	status, _, body := this.apiKeyAuthenticationRead(token)
	this.Equal(http.StatusOK, status, body)

	status, body = this.apiKeyAuthenticationWrite(token)
	this.Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_MarksTheFirstUseAndNotAgainWithinAMinute() {
	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	key, stored := this.apiKeyAuthenticationCreateKey(userID, model.PermissionRead, null.Time{})
	this.False(this.apiKeyAuthenticationLastUsedAt(stored.ID).Valid)

	status, _, body := this.apiKeyAuthenticationRead(key)
	this.Require().Equal(http.StatusOK, status, body)
	firstUse := this.apiKeyAuthenticationLastUsedAt(stored.ID)
	this.Require().True(firstUse.Valid)
	this.WithinDuration(time.Now(), firstUse.Time, time.Minute)

	status, _, body = this.apiKeyAuthenticationRead(key)
	this.Require().Equal(http.StatusOK, status, body)
	this.True(firstUse.Time.Equal(this.apiKeyAuthenticationLastUsedAt(stored.ID).Time))

	_, err := this.Pool.Exec(context.Background(), `UPDATE api_keys SET last_used_at = now() - interval '2 minutes' WHERE id = $1`, stored.ID)
	this.Require().NoError(err)
	status, _, body = this.apiKeyAuthenticationRead(key)
	this.Require().Equal(http.StatusOK, status, body)
	this.WithinDuration(time.Now(), this.apiKeyAuthenticationLastUsedAt(stored.ID).Time, 30*time.Second)
}

func (this *IntegrationSuite) TestApiKeyAuthentication_NeverLogsTheKeyOrItsHash() {
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	userID := this.createActiveUser(apiKeyAuthenticationEmail, model.PermissionWrite, apiKeyAuthenticationPassword)
	readKey, _ := this.apiKeyAuthenticationCreateKey(userID, model.PermissionRead, null.Time{})
	expiredKey, _ := this.apiKeyAuthenticationCreateKey(userID, model.PermissionWrite, null.TimeFrom(time.Now().Add(-time.Second)))
	unknownRandom, err := tokens.Random()
	this.Require().NoError(err)
	unknownKey := "bdk_" + unknownRandom

	this.apiKeyAuthenticationRead(readKey)
	this.apiKeyAuthenticationWrite(readKey)
	this.apiKeyAuthenticationRead(expiredKey)
	this.apiKeyAuthenticationWrite(unknownKey)
	req := httptest.NewRequest(http.MethodPost, "/api/contracts", strings.NewReader(`{"participant":`))
	req.Header.Set("Content-Type", "application/json")
	this.sendApiRequest(req, readKey)

	for _, key := range []string{readKey, expiredKey, unknownKey} {
		hash := tokens.Hash(key)
		for _, secret := range []string{key, strings.TrimPrefix(key, "bdk_"), hex.EncodeToString(hash), base64.StdEncoding.EncodeToString(hash), base64.RawURLEncoding.EncodeToString(hash)} {
			this.NotContains(logged.String(), secret)
		}
		this.False(bytes.Contains(logged.Bytes(), hash))
	}
}
