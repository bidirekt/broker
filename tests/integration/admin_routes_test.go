package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	adminRoutesWriterEmail  = "ana@x.com"
	adminRoutesReaderEmail  = "rui@x.com"
	adminRoutesAdminEmail   = "admin@example.com"
	adminRoutesUserPassword = "admin-routes-correct-horse"
	adminRoutesNoAccess     = "you don&#39;t have access"
)

type adminRoutesRequest struct {
	method string
	path   string
	form   url.Values
}

func (this *IntegrationSuite) adminRoutesSend(request adminRoutesRequest, cookie, csrfToken string) (status int, body string) {
	req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.form.Encode()))
	if request.method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", suitePublicURL)
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	status, _, body = this.sendPageRequest(req, cookie)
	return status, body
}

func (this *IntegrationSuite) TestAdminRoutes_RefuseAWriteOrReadUserSession() {
	this.deleteSuiteWriter()

	for _, user := range []struct {
		email      string
		permission model.Permission
	}{
		{adminRoutesWriterEmail, model.PermissionWrite},
		{adminRoutesReaderEmail, model.PermissionRead},
	} {
		userID := this.createActiveUser(user.email, user.permission, adminRoutesUserPassword)
		cookie := this.signInAsUser(user.email, adminRoutesUserPassword)
		_, _, profile := this.getPage("/profile", cookie)
		csrfToken := this.csrfTokenFrom(profile)

		for _, request := range []adminRoutesRequest{
			{http.MethodGet, "/users", nil},
			{http.MethodGet, "/users/invite", nil},
			{http.MethodPost, "/users/invite", url.Values{"email": {"bob@x.com"}, "permission": {"read"}}},
			{http.MethodGet, fmt.Sprintf("/users/%d/edit", userID), nil},
			{http.MethodPost, fmt.Sprintf("/users/%d/edit", userID), url.Values{"email": {"bob@x.com"}, "permission": {"read"}}},
			{http.MethodPost, fmt.Sprintf("/users/%d/link", userID), url.Values{}},
			{http.MethodPost, fmt.Sprintf("/users/%d/deactivate", userID), url.Values{}},
			{http.MethodPost, fmt.Sprintf("/users/%d/reactivate", userID), url.Values{}},
		} {
			status, body := this.adminRoutesSend(request, cookie, csrfToken)

			this.Equal(http.StatusForbidden, status, string(user.permission)+" "+request.method+" "+request.path)
			this.Contains(body, adminRoutesNoAccess, string(user.permission)+" "+request.method+" "+request.path)
		}
	}

	this.Equal(3, this.countRows("users"))
	this.Equal(0, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestAdminRoutes_RefuseToChangeTheAdminRow() {
	cookie := this.signInAsAdmin()
	_, _, users := this.getPage("/users", cookie)
	csrfToken := this.csrfTokenFrom(users)

	for _, request := range []adminRoutesRequest{
		{http.MethodGet, fmt.Sprintf("/users/%d/edit", this.adminID), nil},
		{http.MethodPost, fmt.Sprintf("/users/%d/edit", this.adminID), url.Values{"email": {"bob@x.com"}, "permission": {"write"}}},
		{http.MethodPost, fmt.Sprintf("/users/%d/edit", this.adminID), url.Values{"email": {adminRoutesAdminEmail}, "permission": {"admin"}}},
		{http.MethodPost, fmt.Sprintf("/users/%d/deactivate", this.adminID), url.Values{}},
		{http.MethodPost, fmt.Sprintf("/users/%d/reactivate", this.adminID), url.Values{}},
		{http.MethodPost, fmt.Sprintf("/users/%d/link", this.adminID), url.Values{}},
	} {
		status, body := this.adminRoutesSend(request, cookie, csrfToken)

		this.Equal(http.StatusForbidden, status, request.method+" "+request.path)
		this.Contains(body, adminRoutesNoAccess, request.method+" "+request.path)
	}

	admin, found := repository.NewUserRepository(this.Pool).FindByID(context.Background(), this.adminID)
	this.Require().True(found)
	this.Equal(adminRoutesAdminEmail, admin.Email)
	this.Equal(model.PermissionAdmin, admin.Permission)
	this.Equal(model.UserActive, admin.State())
	this.Equal(0, this.countRows("one_time_links"))
}
