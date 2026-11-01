package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	apiPermissionReaderEmail    = "rui@x.com"
	apiPermissionWriterEmail    = "wanda@x.com"
	apiPermissionPassword       = "permission-correct-horse-battery"
	apiPermissionForbidden      = `{"message":"forbidden"}`
	apiPermissionMalformedBody  = `{"participant":`
	apiPermissionProviderSource = "orders.yaml"
	apiPermissionProviderYAML   = `provides:
  rest:
    /orders:
      get:
        responses:
          200: Order
schemas:
  Order:
    type: object
    properties:
      id:
        type: string
`
)

type apiPermissionRoute struct {
	method string
	path   string
	body   string
}

func (this *IntegrationSuite) apiPermissionSend(route apiPermissionRoute, token string) (status int, body string) {
	req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
	req.Header.Set("Content-Type", "application/json")

	status, _, body = this.sendApiRequest(req, token)

	return status, body
}

func (this *IntegrationSuite) apiPermissionWriteRoutes() []apiPermissionRoute {
	return []apiPermissionRoute{
		{http.MethodPost, "/api/contracts", this.publishBody("orders", "v2", contractFragment{apiPermissionProviderSource, apiPermissionProviderYAML})},
		{http.MethodPost, "/api/deployments", `{"participant":"orders","version":"v1","environment":"production"}`},
		{http.MethodPost, "/api/participants", `{"participant":"billing"}`},
		{http.MethodPost, "/api/participants/rename", `{"oldName":"orders","newName":"sales"}`},
		{http.MethodPost, "/api/environments", `{"environment":"staging"}`},
	}
}

func (this *IntegrationSuite) apiPermissionSeed() {
	for _, request := range []struct{ path, body string }{
		{"/api/participants", `{"participant":"orders"}`},
		{"/api/contracts", this.publishBody("orders", "v1", contractFragment{apiPermissionProviderSource, apiPermissionProviderYAML})},
		{"/api/environments", `{"environment":"production"}`},
	} {
		status, body := this.post(request.path, request.body)
		this.Require().Equal(http.StatusOK, status, body)
	}
}

func (this *IntegrationSuite) TestApiPermission_ReadTokenIsForbiddenOnEveryWriteRoute() {
	this.apiPermissionSeed()
	this.createActiveUser(apiPermissionReaderEmail, model.PermissionRead, apiPermissionPassword)
	token := this.signInToApi(apiPermissionReaderEmail, apiPermissionPassword)

	for _, route := range this.apiPermissionWriteRoutes() {
		this.Run(route.path, func() {
			status, body := this.apiPermissionSend(route, token)
			this.Equal(http.StatusForbidden, status)
			this.JSONEq(apiPermissionForbidden, body)
		})
	}

	this.Equal(1, this.countRows("participants"))
	this.Equal(1, this.countRows("environments"))
	this.Equal(1, this.countRows("contract_versions"))
	this.Equal(0, this.countRows("deployments"))
}

func (this *IntegrationSuite) TestApiPermission_ReadTokenIsForbiddenBeforeTheBodyIsRead() {
	this.createActiveUser(apiPermissionReaderEmail, model.PermissionRead, apiPermissionPassword)
	token := this.signInToApi(apiPermissionReaderEmail, apiPermissionPassword)

	for _, route := range this.apiPermissionWriteRoutes() {
		this.Run(route.path, func() {
			route.body = apiPermissionMalformedBody

			status, body := this.apiPermissionSend(route, token)
			this.Equal(http.StatusForbidden, status)
			this.JSONEq(apiPermissionForbidden, body)
		})
	}
}

func (this *IntegrationSuite) TestApiPermission_ReadTokenIsAllowedOnEveryReadRoute() {
	this.apiPermissionSeed()
	this.createActiveUser(apiPermissionReaderEmail, model.PermissionRead, apiPermissionPassword)
	token := this.signInToApi(apiPermissionReaderEmail, apiPermissionPassword)

	validateBody, err := json.Marshal(map[string]any{
		"participant": "orders",
		"environment": "production",
		"contracts":   []contractFragment{{apiPermissionProviderSource, apiPermissionProviderYAML}},
	})
	this.Require().NoError(err)

	for _, route := range []apiPermissionRoute{
		{http.MethodGet, "/api/participants", ""},
		{http.MethodGet, "/api/environments", ""},
		{http.MethodPost, "/api/contracts/validate", string(validateBody)},
		{http.MethodPost, "/api/can-i-deploy", `{"participant":"orders","version":"v1","environment":"production"}`},
	} {
		this.Run(route.path, func() {
			status, body := this.apiPermissionSend(route, token)
			this.Equal(http.StatusOK, status, body)
		})
	}
}

func (this *IntegrationSuite) TestApiPermission_DowngradedUserIsForbiddenAtOnceWithAnExistingToken() {
	userID := this.createActiveUser(apiPermissionWriterEmail, model.PermissionWrite, apiPermissionPassword)
	token := this.signInToApi(apiPermissionWriterEmail, apiPermissionPassword)
	route := apiPermissionRoute{http.MethodPost, "/api/participants", `{"participant":"billing"}`}

	status, body := this.apiPermissionSend(route, token)
	this.Require().Equal(http.StatusOK, status, body)

	_, err := this.Pool.Exec(context.Background(), `UPDATE users SET permission = 'read' WHERE id = $1`, userID)
	this.Require().NoError(err)

	route.body = `{"participant":"shipping"}`
	status, body = this.apiPermissionSend(route, token)
	this.Equal(http.StatusForbidden, status)
	this.JSONEq(apiPermissionForbidden, body)
	this.Equal(1, this.countRows("participants"))
}
