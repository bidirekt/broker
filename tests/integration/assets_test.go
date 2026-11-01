package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (this *IntegrationSuite) fetchAsset(path string) (status int, contentType string, body string) {
	req := httptest.NewRequest("GET", path, nil)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header.Get("Content-Type"), string(bytes)
}

func (this *IntegrationSuite) TestHappyPath_HtmxIsServedAsJavaScript() {
	status, contentType, body := this.fetchAsset("/assets/htmx.min.js")
	this.Equal(http.StatusOK, status)
	this.Contains(contentType, "javascript")
	this.Contains(body, "htmx")
}

func (this *IntegrationSuite) TestHappyPath_HtmxLicenseIsServed() {
	status, _, body := this.fetchAsset("/assets/htmx.LICENSE")
	this.Equal(http.StatusOK, status)
	this.Contains(body, "Zero-Clause BSD")
}

func (this *IntegrationSuite) TestHappyPath_AppScriptConfiguresHtmx() {
	status, contentType, body := this.fetchAsset("/assets/app.js")
	this.Equal(http.StatusOK, status)
	this.Contains(contentType, "javascript")
	this.Contains(body, "htmx.config.includeIndicatorStyles = false")
	this.Contains(body, "htmx.config.historyCacheSize = 0")
	this.Contains(body, "htmx.config.responseHandling")
}

func (this *IntegrationSuite) TestHappyPath_StylesheetIsServedAsCSS() {
	status, contentType, _ := this.fetchAsset("/assets/app.css")
	this.Equal(http.StatusOK, status)
	this.Contains(contentType, "text/css")
}
