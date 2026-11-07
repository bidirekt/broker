package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/bearer"
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
	{http.MethodPost, "/api/environments"},
	{http.MethodPost, "/api/environments/rename"},
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
	token, _, err := bearer.Issue(this.Components.Settings, userID, time.Now().Add(-16*time.Minute))
	this.Require().NoError(err)

	this.apiAuthenticationAssertRefused(token)
}

func (this *IntegrationSuite) TestApiAuthentication_RefusesATokenOfAnUnknownUser() {
	token, _, err := bearer.Issue(this.Components.Settings, 999999, time.Now())
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

func (this *IntegrationSuite) TestApiAuthentication_RefusesASessionCookieOnAnotherCaseOfTheApiPath() {
	this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	cookie := this.signInAsUser(apiAuthenticationEmail, apiAuthenticationPassword)
	_, _, profile := this.getPage("/profile", cookie)
	csrfToken := this.csrfTokenFrom(profile)

	for _, path := range []string{"/API/participants", "/Api/Participants"} {
		this.Run("GET "+path, func() {
			status, _, _ := this.getPage(path, cookie)
			this.Equal(http.StatusNotFound, status)
		})

		this.Run("POST "+path, func() {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"participant":"pets"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", suitePublicURL)
			req.Header.Set("X-CSRF-Token", csrfToken)

			status, _, _ := this.sendPageRequest(req, cookie)
			this.Equal(http.StatusNotFound, status)
			this.Equal(0, this.countRows("participants"))
		})
	}
}

func (this *IntegrationSuite) TestApiAuthentication_SendsATokenOnAnotherCaseOfTheApiPathToSignIn() {
	for _, path := range []string{"/API/participants", "/Api/Participants"} {
		this.Run("GET "+path, func() {
			status, header, _ := this.getWithToken(path, this.writerToken)
			this.Equal(http.StatusSeeOther, status)
			this.Equal("/sign-in", header.Get("Location"))
		})

		this.Run("POST "+path, func() {
			status, header, _ := this.postWithToken(path, this.writerToken, `{"participant":"pets"}`)
			this.Equal(http.StatusSeeOther, status)
			this.Equal("/sign-in", header.Get("Location"))
			this.Equal(0, this.countRows("participants"))
		})
	}
}

func (this *IntegrationSuite) TestApiAuthentication_SignInWithATrailingSlashIssuesNoToken() {
	this.createActiveUser(apiAuthenticationEmail, model.PermissionWrite, apiAuthenticationPassword)
	requestBody, err := json.Marshal(map[string]string{"email": apiAuthenticationEmail, "password": apiAuthenticationPassword})
	this.Require().NoError(err)

	status, header, body := this.postWithToken("/api/sign-in/", "", string(requestBody))
	this.Equal(http.StatusUnauthorized, status)
	this.Equal("Bearer", header.Get("WWW-Authenticate"))
	this.JSONEq(apiAuthenticationUnauthenticated, body)
}

func (this *IntegrationSuite) TestApiAuthentication_TheParticipantRenameRouteIsNotFound() {
	status, response := this.post("/api/participants", `{"participant":"pets"}`)
	this.Require().Equal(http.StatusOK, status, response)

	status, _, _ = this.postWithToken("/api/participants/rename", this.writerToken, `{"oldName":"pets","newName":"orders"}`)

	this.Equal(http.StatusNotFound, status)
	_, _, participants := this.getWithToken("/api/participants", this.writerToken)
	this.Contains(participants, `"pets"`)
	this.NotContains(participants, `"orders"`)
}
