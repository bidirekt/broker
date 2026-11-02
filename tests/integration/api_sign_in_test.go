package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/bearer"
	"github.com/bidirekt/broker/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

const (
	apiSignInEmail              = "ana@x.com"
	apiSignInPassword           = "ana-correct-horse-battery"
	apiSignInAdminUsername      = "admin"
	apiSignInAdminPassword      = "integration-admin-password"
	apiSignInInvalidCredentials = `{"message":"invalid credentials"}`
	apiSignInInvalidInput       = `{"message":"sign in invalid input"}`
	apiSignInForbidden          = `{"message":"forbidden"}`
	apiSignInForeignOrigin      = "https://evil.example"
)

func (this *IntegrationSuite) apiSignInBody(email, password string) string {
	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	this.Require().NoError(err)

	return string(body)
}

func (this *IntegrationSuite) TestApiSignIn_ActiveUserGetsAToken() {
	userID := this.createActiveUser(apiSignInEmail, model.PermissionWrite, apiSignInPassword)

	status, body := this.post("/api/sign-in", this.apiSignInBody(apiSignInEmail, apiSignInPassword))
	this.Require().Equal(http.StatusOK, status, body)

	var response struct {
		Message   string `json:"message"`
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))
	this.Equal("signed in", response.Message)

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(response.Token, claims,
		func(*jwt.Token) (any, error) { return bearer.SigningKey, nil },
		jwt.WithValidMethods([]string{"HS256"}),
	)
	this.Require().NoError(err)
	this.Equal(strconv.FormatInt(userID, 10), claims["sub"])

	expiresAt, err := time.Parse(time.RFC3339, response.ExpiresAt)
	this.Require().NoError(err)
	this.Equal(time.UTC, expiresAt.Location())
	this.Equal(response.ExpiresAt, expiresAt.UTC().Format(time.RFC3339))
	this.WithinDuration(time.Now().Add(15*time.Minute), expiresAt, 5*time.Second)
}

func (this *IntegrationSuite) TestApiSignIn_RefusesWithOneAnswer() {
	this.createActiveUser(apiSignInEmail, model.PermissionWrite, apiSignInPassword)
	invitedID := this.createActiveUser("invited@x.com", model.PermissionWrite, apiSignInPassword)
	this.setUserState(invitedID, model.UserInvited)
	deactivatedID := this.createActiveUser("deactivated@x.com", model.PermissionWrite, apiSignInPassword)
	this.setUserState(deactivatedID, model.UserDeactivated)

	refused := map[string]string{
		"wrong password":    this.apiSignInBody(apiSignInEmail, "ana-wrong-horse-battery"),
		"unknown email":     this.apiSignInBody("nobody@x.com", apiSignInPassword),
		"invited user":      this.apiSignInBody("invited@x.com", apiSignInPassword),
		"deactivated user":  this.apiSignInBody("deactivated@x.com", apiSignInPassword),
		"admin credentials": this.apiSignInBody(apiSignInAdminUsername, apiSignInAdminPassword),
	}

	for name, requestBody := range refused {
		this.Run(name, func() {
			status, body := this.post("/api/sign-in", requestBody)
			this.Equal(http.StatusUnauthorized, status)
			this.JSONEq(apiSignInInvalidCredentials, body)
		})
	}
}

func (this *IntegrationSuite) TestApiSignIn_RejectsInvalidInput() {
	invalid := map[string]string{
		"missing body":   "",
		"malformed JSON": `{"email":`,
		"empty email":    this.apiSignInBody("", apiSignInPassword),
		"empty password": this.apiSignInBody(apiSignInEmail, ""),
	}

	for name, requestBody := range invalid {
		this.Run(name, func() {
			status, body := this.post("/api/sign-in", requestBody)
			this.Equal(http.StatusBadRequest, status)
			this.JSONEq(apiSignInInvalidInput, body)
		})
	}
}

func (this *IntegrationSuite) apiSignInSend(contentType, origin, requestBody string) (status int, body string) {
	req := httptest.NewRequest("POST", "/api/sign-in", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", contentType)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	status, _, body = this.sendApiRequest(req, "")
	return status, body
}

func (this *IntegrationSuite) TestApiSignIn_ForeignOriginIsForbidden() {
	this.createActiveUser(apiSignInEmail, model.PermissionWrite, apiSignInPassword)

	status, body := this.apiSignInSend("application/json", apiSignInForeignOrigin, this.apiSignInBody(apiSignInEmail, apiSignInPassword))
	this.Equal(http.StatusForbidden, status)
	this.JSONEq(apiSignInForbidden, body)
}

func (this *IntegrationSuite) TestApiSignIn_PlainTextBodyIsInvalidInput() {
	this.createActiveUser(apiSignInEmail, model.PermissionWrite, apiSignInPassword)

	status, body := this.apiSignInSend("text/plain", "", this.apiSignInBody(apiSignInEmail, apiSignInPassword))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(apiSignInInvalidInput, body)
}

func (this *IntegrationSuite) TestApiSignIn_JsonMediaTypeMatchesCaseInsensitively() {
	this.createActiveUser(apiSignInEmail, model.PermissionWrite, apiSignInPassword)

	status, body := this.apiSignInSend("Application/JSON; charset=utf-8", suitePublicURL, this.apiSignInBody(apiSignInEmail, apiSignInPassword))
	this.Equal(http.StatusOK, status, body)
}
