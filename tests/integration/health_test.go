package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (this *IntegrationSuite) health(method string) (status int, response string) {
	req := httptest.NewRequest(method, "/health", nil)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, string(bytes)
}

func (this *IntegrationSuite) TestHappyPath_HealthReportsStatusAndVersions() {
	status, body := this.health("GET")
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"status":"ok","brokerVersion":"dev","apiVersion":1}`, body)
}

func (this *IntegrationSuite) TestUnhappyPath_PostHealthIsNotRouted() {
	status, body := this.health("POST")
	this.Equal(http.StatusMethodNotAllowed, status)
	this.JSONEq(`{"message":"Method Not Allowed"}`, body)
}
