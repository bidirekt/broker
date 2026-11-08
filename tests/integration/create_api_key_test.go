package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

const (
	createAPIKeyPath           = "/profile/api-keys/new"
	createAPIKeyReaderEmail    = "reader@x.com"
	createAPIKeyReaderPassword = "reader-correct-horse-battery"
	createAPIKeyName           = "payments CI"
	createAPIKeyReadRadio      = `<input type="radio" name="permission" value="read" required`
	createAPIKeyWriteRadio     = `<input type="radio" name="permission" value="write" required`
)

var createAPIKeyInPage = regexp.MustCompile(`value="(bdk_([A-Za-z0-9_-]{43}))"`)

func createAPIKeyFieldError(field string) string {
	return `<p class="err" id="` + field + `-err"><svg`
}

func createAPIKeyFieldMessage(message string) string {
	return `</svg><span>` + message + `</span></p>`
}

func (this *IntegrationSuite) createAPIKeyPost(cookie string, form url.Values) (status int, body string) {
	_, _, page := this.getPage(createAPIKeyPath, cookie)
	status, _, body = this.postForm(createAPIKeyPath, cookie, this.csrfTokenFrom(page), form)

	return status, body
}

func (this *IntegrationSuite) createAPIKeyStored() (userID int64, keyHash []byte, prefix, name string, permission model.Permission, expiresAt null.Time) {
	err := this.Pool.QueryRow(context.Background(),
		`SELECT user_id, key_hash, display_prefix, name, permission, expires_at FROM api_keys`,
	).Scan(&userID, &keyHash, &prefix, &name, &permission, &expiresAt)
	this.Require().NoError(err)

	return userID, keyHash, prefix, name, permission, expiresAt
}

func (this *IntegrationSuite) TestCreateAPIKey_FormOffersBothPermissionsWithWriteAndNinetyDaysChecked() {
	status, header, body := this.getPage(createAPIKeyPath, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<form class="sheet formcard" method="post" action="/profile/api-keys/new">`)
	this.Contains(body, `name="name" type="text" autocomplete="off" value="" required autofocus>`)
	this.Contains(body, createAPIKeyReadRadio+`>`)
	this.Contains(body, createAPIKeyWriteRadio+` checked>`)
	this.Contains(body, `<input type="radio" name="expiration" value="30" required>`)
	this.Contains(body, `<input type="radio" name="expiration" value="90" required checked>`)
	this.Contains(body, `<input type="radio" name="expiration" value="365" required>`)
	this.Contains(body, `<input type="radio" name="expiration" value="never" required>`)
	this.NotContains(body, `aria-invalid`)
}

func (this *IntegrationSuite) TestCreateAPIKey_ReadUserIsOfferedOnlyRead() {
	this.createActiveUser(createAPIKeyReaderEmail, model.PermissionRead, createAPIKeyReaderPassword)

	status, _, body := this.getPage(createAPIKeyPath, this.signInAsUser(createAPIKeyReaderEmail, createAPIKeyReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, createAPIKeyReadRadio+` checked>`)
	this.NotContains(body, createAPIKeyWriteRadio)
}

func (this *IntegrationSuite) TestCreateAPIKey_CreatesTheKeyAndShowsItOnlyInTheResponse() {
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	cookie := this.signInAsAdmin()
	status, body := this.createAPIKeyPost(cookie, url.Values{
		"name":       {createAPIKeyName},
		"permission": {"write"},
		"expiration": {"never"},
	})

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, "<h1>API key created</h1>")
	this.Contains(body, "Copy it now, it won't be shown again.")
	this.Contains(body, `<a class="btn primary" href="/profile">Back to profile</a>`)
	match := createAPIKeyInPage.FindStringSubmatch(body)
	this.Require().Len(match, 3, "page shows no API key")
	key, random := match[1], match[2]
	this.Contains(body, `<div class="secret">`)
	this.Contains(body, `<input class="copy" type="text" readonly value="`+key+`" aria-label="API key">`)

	userID, keyHash, prefix, name, permission, expiresAt := this.createAPIKeyStored()
	expectedHash := sha256.Sum256([]byte(key))
	this.Equal(this.adminID, userID)
	this.Equal(expectedHash[:], keyHash)
	this.Equal("bdk_"+random[:8], prefix)
	this.Equal(createAPIKeyName, name)
	this.Equal(model.PermissionWrite, permission)
	this.False(expiresAt.Valid)

	_, _, profile := this.getPage("/profile", cookie)
	this.NotContains(profile, key)
	this.NotContains(profile, random)
	_, _, form := this.getPage(createAPIKeyPath, cookie)
	this.NotContains(form, key)
	this.NotContains(form, random)
	this.NotContains(logged.String(), random)
}

func (this *IntegrationSuite) TestCreateAPIKey_ExpirationChoicesSetTheExpiry() {
	cookie := this.signInAsAdmin()

	for expiration, days := range map[string]int{"30": 30, "90": 90, "365": 365} {
		_, err := this.Pool.Exec(context.Background(), `DELETE FROM api_keys`)
		this.Require().NoError(err)

		status, body := this.createAPIKeyPost(cookie, url.Values{
			"name":       {createAPIKeyName},
			"permission": {"read"},
			"expiration": {expiration},
		})

		this.Require().Equal(http.StatusOK, status, expiration+" "+body)
		_, _, _, _, permission, expiresAt := this.createAPIKeyStored()
		this.Equal(model.PermissionRead, permission, expiration)
		this.Require().True(expiresAt.Valid, expiration)
		this.WithinDuration(time.Now().Add(time.Duration(days)*24*time.Hour), expiresAt.Time, time.Minute, expiration)
	}
}

func (this *IntegrationSuite) TestCreateAPIKey_ReadUserForgingWriteGetsTheFieldErrorAndNothingIsCreated() {
	this.createActiveUser(createAPIKeyReaderEmail, model.PermissionRead, createAPIKeyReaderPassword)
	cookie := this.signInAsUser(createAPIKeyReaderEmail, createAPIKeyReaderPassword)

	status, body := this.createAPIKeyPost(cookie, url.Values{
		"name":       {createAPIKeyName},
		"permission": {"write"},
		"expiration": {"30"},
	})

	this.Equal(http.StatusBadRequest, status)
	this.Contains(body, createAPIKeyFieldError("permission"))
	this.Contains(body, createAPIKeyFieldMessage("you can&#39;t give more than your own permission"))
	this.Contains(body, `value="`+createAPIKeyName+`"`)
	this.Contains(body, `<input type="radio" name="expiration" value="30" required checked>`)
	this.Equal(0, this.countRows("api_keys"))
}

func (this *IntegrationSuite) TestCreateAPIKey_FieldErrorsKeepTheValuesAndCreateNothing() {
	cookie := this.signInAsAdmin()
	cases := []struct {
		name       string
		permission string
		expiration string
		field      string
		message    string
	}{
		{"", "read", "90", "name", "name is required"},
		{strings.Repeat("é", 101), "read", "90", "name", "name has at most 100 characters"},
		{createAPIKeyName, "admin", "90", "permission", "choose read or write"},
		{createAPIKeyName, "", "90", "permission", "choose read or write"},
		{createAPIKeyName, "read", "7", "expiration", "choose 30, 90, 365 days or never"},
		{createAPIKeyName, "read", "", "expiration", "choose 30, 90, 365 days or never"},
	}

	for _, test := range cases {
		status, body := this.createAPIKeyPost(cookie, url.Values{
			"name":       {test.name},
			"permission": {test.permission},
			"expiration": {test.expiration},
		})

		this.Equal(http.StatusBadRequest, status, test.message)
		this.Contains(body, createAPIKeyFieldError(test.field), test.message)
		this.Contains(body, createAPIKeyFieldMessage(test.message), test.message)
		this.Contains(body, `aria-describedby="`+test.field+`-err"`, test.message)
		this.Contains(body, `value="`+test.name+`"`, test.message)
		this.NotContains(body, "<h1>API key created</h1>", test.message)
	}

	this.Equal(0, this.countRows("api_keys"))
}

func (this *IntegrationSuite) TestCreateAPIKey_NameOfExactlyAHundredCharactersIsAccepted() {
	status, body := this.createAPIKeyPost(this.signInAsAdmin(), url.Values{
		"name":       {strings.Repeat("é", 100)},
		"permission": {"read"},
		"expiration": {"90"},
	})

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal(1, this.countRows("api_keys"))
}
