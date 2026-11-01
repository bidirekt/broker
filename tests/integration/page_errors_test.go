package integration_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

const (
	pageContentSecurityPolicy = "default-src 'self'; frame-ancestors 'none'"
	pageErrorsSessionCookie   = "page-errors-session-cookie"
	pageErrorsCSRFToken       = "page-errors-csrf-token"
)

func (this *IntegrationSuite) requestPage(method, path string) (status int, header http.Header, body string) {
	return this.sendPageErrorsRequest(httptest.NewRequest(method, path, nil))
}

func (this *IntegrationSuite) sendPageErrorsRequest(req *http.Request) (status int, header http.Header, body string) {
	idHash := sha256.Sum256([]byte(pageErrorsSessionCookie))
	sessions := repository.NewSessionRepository(this.Pool)
	if _, found := sessions.FindByIDHash(context.Background(), idHash[:]); !found {
		sessions.Create(context.Background(), &model.Session{IDHash: idHash[:], CSRFToken: pageErrorsCSRFToken}, "")
	}

	req.AddCookie(&http.Cookie{Name: "bidirekt_session", Value: pageErrorsSessionCookie})

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) TestUnhappyPath_UnknownPageAnswers404AsHTMLWithCSP() {
	status, header, body := this.requestPage("GET", "/no-such-page")
	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal(pageContentSecurityPolicy, header.Get("Content-Security-Policy"))
	this.Contains(body, "<!doctype html>")
	this.Contains(body, "<h1>Not Found</h1>")
	this.Contains(body, `src="/assets/htmx.min.js"`)
	this.NotContains(body, "<style")
	this.Contains(body, `action="/sign-out"`)
	this.Contains(body, pageErrorsCSRFToken)
}

func (this *IntegrationSuite) TestUnhappyPath_UnknownApiPathAnswers404AsJSON() {
	status, header, body := this.requestPage("GET", "/api/no-such-route")
	this.Equal(http.StatusNotFound, status)
	this.Contains(header.Get("Content-Type"), "application/json")
	this.Empty(header.Get("Content-Security-Policy"))
	this.JSONEq(`{"message":"Not Found"}`, body)
}

func (this *IntegrationSuite) TestUnhappyPath_PageFailureAnswers500AsSomethingWentWrong() {
	this.Components.Server.Get("/__failing_page", func(fiber.Ctx) error {
		return errors.New("database exploded")
	})

	status, header, body := this.requestPage("GET", "/__failing_page")
	this.Equal(http.StatusInternalServerError, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal(pageContentSecurityPolicy, header.Get("Content-Security-Policy"))
	this.Contains(body, "something went wrong")
	this.NotContains(body, "database exploded")
}

func (this *IntegrationSuite) TestUnhappyPath_PageFiberErrorKeepsItsCodeAndMessage() {
	this.Components.Server.Get("/__conflicting_page", func(fiber.Ctx) error {
		return fiber.NewError(http.StatusConflict, "that email is already taken")
	})

	status, header, body := this.requestPage("GET", "/__conflicting_page")
	this.Equal(http.StatusConflict, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "that email is already taken")
}

func (this *IntegrationSuite) TestUnhappyPath_HtmxPagePostRefusedWith403DoesNotPushTheFormURL() {
	req := httptest.NewRequest("POST", "/users/invite", nil)
	req.Header.Set("HX-Request", "true")

	status, header, body := this.sendPageErrorsRequest(req)
	this.Equal(http.StatusForbidden, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "<h1>forbidden</h1>")
	this.Equal("false", header.Get("HX-Push-Url"))
	this.Equal("no-store", header.Get("Cache-Control"))
}

func (this *IntegrationSuite) TestUnhappyPath_HtmxPagePostFailureAnswers500WithoutPushingTheFormURL() {
	this.Components.Server.Post("/__failing_form", func(fiber.Ctx) error {
		return errors.New("database exploded")
	})

	req := httptest.NewRequest("POST", "/__failing_form", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("X-CSRF-Token", pageErrorsCSRFToken)

	status, header, body := this.sendPageErrorsRequest(req)
	this.Equal(http.StatusInternalServerError, status)
	this.Contains(body, "something went wrong")
	this.Equal("false", header.Get("HX-Push-Url"))
}
