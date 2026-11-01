package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
)

const (
	apiAuthenticationEmail           = "carla@x.com"
	apiAuthenticationPassword        = "carla-correct-horse-battery"
	apiAuthenticationUnauthenticated = `{"message":"unauthenticated"}`
)

type apiAuthenticationRoute struct {
	method string
	path   string
}

var apiAuthenticationRoutes = []apiAuthenticationRoute{
	{http.MethodGet, "/api/participants"},
	{http.MethodGet, "/api/environments"},
	{http.MethodPost, "/api/contracts/validate"},
	{http.MethodPost, "/api/can-i-deploy"},
	{http.MethodPost, "/api/contracts"},
	{http.MethodPost, "/api/deployments"},
	{http.MethodPost, "/api/participants"},
	{http.MethodPost, "/api/participants/rename"},
	{http.MethodPost, "/api/environments"},
}

func (this *IntegrationSuite) apiAuthenticationRequest(route apiAuthenticationRoute) *http.Request {
	req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	return req
}

func (this *IntegrationSuite) apiAuthenticationAssertRefused(token string) {
	for _, route := range apiAuthenticationRoutes {
		this.Run(route.method+" "+route.path, func() {
			status, header, body := this.sendApiRequest(this.apiAuthenticationRequest(route), token)
			this.Equal(http.StatusUnauthorized, status)
			this.Equal("Bearer", header.Get("WWW-Authenticate"))
			this.JSONEq(apiAuthenticationUnauthenticated, body)
		})
	}
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesARequestWithoutAToken() {
	this.apiAuthenticationAssertRefused("")
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesAMalformedToken() {
	this.apiAuthenticationAssertRefused("not-a-jwt")
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesAnExpiredToken() {
	userID := this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	token, _, err := components.IssueToken(this.Components.Settings, userID, time.Now().Add(-16*time.Minute))
	this.Require().NoError(err)

	this.apiAuthenticationAssertRefused(token)
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesATokenOfAnUnknownUser() {
	token, _, err := components.IssueToken(this.Components.Settings, 999999, time.Now())
	this.Require().NoError(err)

	this.apiAuthenticationAssertRefused(token)
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesADeactivatedUsersUnexpiredToken() {
	userID := this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	token := this.signInToApi(apiAuthenticationEmail, apiAuthenticationPassword)
	this.setUserState(userID, model.UserDeactivated)

	this.apiAuthenticationAssertRefused(token)
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesATokenWithoutTheBearerScheme() {
	this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	token := this.signInToApi(apiAuthenticationEmail, apiAuthenticationPassword)

	for _, route := range apiAuthenticationRoutes {
		this.Run(route.method+" "+route.path, func() {
			req := this.apiAuthenticationRequest(route)
			req.Header.Set("Authorization", token)

			status, header, body := this.sendApiRequest(req, "")
			this.Equal(http.StatusUnauthorized, status)
			this.Equal("Bearer", header.Get("WWW-Authenticate"))
			this.JSONEq(apiAuthenticationUnauthenticated, body)
		})
	}
}

func (this *IntegrationSuite) TestApiAuthentication_IgnoresASessionCookie() {
	this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	cookie := this.signInAsUser(apiAuthenticationEmail, apiAuthenticationPassword)

	for _, route := range apiAuthenticationRoutes {
		this.Run(route.method+" "+route.path, func() {
			req := this.apiAuthenticationRequest(route)
			req.AddCookie(&http.Cookie{Name: "bidirekt_session", Value: cookie})

			status, header, body := this.sendApiRequest(req, "")
			this.Equal(http.StatusUnauthorized, status)
			this.Equal("Bearer", header.Get("WWW-Authenticate"))
			this.JSONEq(apiAuthenticationUnauthenticated, body)
		})
	}
}

func (this *IntegrationSuite) TestApiAuthentication_LeavesSignInOpen() {
	this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	requestBody, err := json.Marshal(map[string]string{"email": apiAuthenticationEmail, "password": apiAuthenticationPassword})
	this.Require().NoError(err)

	status, header, body := this.postWithToken("/api/sign-in", "", string(requestBody))
	this.Equal(http.StatusOK, status, body)
	this.Empty(header.Get("WWW-Authenticate"))
}

func (this *IntegrationSuite) TestApiAuthentication_LeavesHealthOpenOnApiVersion2() {
	status, _, body := this.getWithToken("/health", "")
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"status":"ok","brokerVersion":"dev","apiVersion":2}`, body)
}
