package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	adminRoutesUserEmail    = "ana@x.com"
	adminRoutesUserPassword = "admin-routes-correct-horse"
	adminRoutesNoAccess     = "you don&#39;t have access"
)

func (this *IntegrationSuite) TestAdminRoutes_RefuseAUserSession() {
	this.deleteSuiteWriter()

	userID := this.createActiveUser(adminRoutesUserEmail, model.PermissionWrite, adminRoutesUserPassword)
	cookie := this.signInAsUser(adminRoutesUserEmail, adminRoutesUserPassword)
	_, _, profile := this.getPage("/profile", cookie)
	csrfToken := this.csrfTokenFrom(profile)

	routes := []struct {
		method string
		path   string
		form   url.Values
	}{
		{http.MethodGet, "/users", nil},
		{http.MethodGet, "/users/invite", nil},
		{http.MethodPost, "/users/invite", url.Values{"email": {"bob@x.com"}, "permission": {"read"}}},
		{http.MethodGet, fmt.Sprintf("/users/%d/edit", userID), nil},
		{http.MethodPost, fmt.Sprintf("/users/%d/edit", userID), url.Values{"email": {"bob@x.com"}, "permission": {"read"}}},
		{http.MethodPost, fmt.Sprintf("/users/%d/link", userID), url.Values{}},
		{http.MethodPost, fmt.Sprintf("/users/%d/deactivate", userID), url.Values{}},
		{http.MethodPost, fmt.Sprintf("/users/%d/reactivate", userID), url.Values{}},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.form.Encode()))
		if route.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", suitePublicURL)
			req.Header.Set("X-CSRF-Token", csrfToken)
		}

		status, _, body := this.sendPageRequest(req, cookie)

		this.Equal(http.StatusForbidden, status, route.method+" "+route.path)
		this.Contains(body, adminRoutesNoAccess, route.method+" "+route.path)
	}

	this.Equal(1, this.countRows("users"))
	this.Equal(0, this.countRows("one_time_links"))
}
