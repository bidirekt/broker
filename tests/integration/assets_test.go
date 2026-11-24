package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	status, contentType, _ := this.fetchAsset("/assets/bundle.css")
	this.Equal(http.StatusOK, status)
	this.Contains(contentType, "text/css")
}

func (this *IntegrationSuite) TestUnhappyPath_AssetsAreServedOnlyAtTheirExactPath() {
	req := httptest.NewRequest("GET", "/ASSETS/bundle.css", nil)
	req.AddCookie(&http.Cookie{Name: "bidirekt_session", Value: this.signInAsAdmin()})

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	_ = resp.Body.Close()

	this.Equal(http.StatusNotFound, resp.StatusCode)
}

var restSectionRules = []string{
	".ep{", ".group-label+.ep{", ".ep-title{", ".meth{", ".prop{", ".ix{", ".ix-name{", ".brk{", ".brk svg{",
	".places{", ".places li{", ".places li:first-child{", ".places .at{", ".places .cf{",
}

func ruleLines(stylesheet string) []string {
	var rules []string
	for _, line := range strings.Split(stylesheet, "\n") {
		if strings.HasPrefix(line, ".") {
			rules = append(rules, line)
		}
	}
	return rules
}

func (this *IntegrationSuite) TestHappyPath_RestSectionStylesheetIsServedAsCSSWithTheRulesOnlyItsTemplateUses() {
	status, contentType, body := this.fetchAsset("/assets/protocols/endpoints.css")
	this.Equal(http.StatusOK, status)
	this.Contains(contentType, "text/css")

	rules := ruleLines(body)
	this.Require().Len(rules, len(restSectionRules))
	for index, selector := range restSectionRules {
		this.True(strings.HasPrefix(rules[index], selector), rules[index])
	}
}

func (this *IntegrationSuite) TestHappyPath_StylesheetKeepsOnlyTheRulesOtherPagesShare() {
	_, _, body := this.fetchAsset("/assets/bundle.css")

	for _, rule := range ruleLines(body) {
		for _, selector := range restSectionRules {
			this.False(strings.HasPrefix(rule, selector), rule)
		}
	}
	for _, shared := range []string{".group-label{", ".fixes{", ".fix{", ".badge{", ".muted{"} {
		this.Contains(body, "\n"+shared)
	}
}

func (this *IntegrationSuite) TestUnhappyPath_OnlyRegisteredSectionsHaveAStylesheet() {
	status, _, _ := this.fetchAsset("/assets/protocols/probes.css")
	this.Equal(http.StatusNotFound, status)
}
