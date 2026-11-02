package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Authorization", "Bearer "+this.writerToken)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) sendRawPageErrorsRequest(raw string) (status int, header http.Header, body string) {
	client, server := net.Pipe()
	served := make(chan error, 1)
	go func() { served <- this.Components.Server.Server().ServeConn(server) }()
	go func() { _, _ = client.Write([]byte(raw)) }()

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	this.Require().NoError(err)
	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)
	_ = client.Close()
	<-served

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

func (this *IntegrationSuite) TestUnhappyPath_OversizedApiRequestHeadAnswers431AsJSONWithoutARedirect() {
	status, header, body := this.sendRawPageErrorsRequest(
		"GET /api/participants HTTP/1.1\r\nHost: localhost:8080\r\nX-Oversized: " + strings.Repeat("a", 9*1024) + "\r\n\r\n",
	)
	this.Equal(http.StatusRequestHeaderFieldsTooLarge, status)
	this.Contains(header.Get("Content-Type"), "application/json")
	this.Empty(header.Get("Location"))
	this.JSONEq(`{"message":"Request Header Fields Too Large"}`, body)
}

func (this *IntegrationSuite) TestUnhappyPath_ApiRequestWithABadContentLengthAnswers400AsJSONWithoutARedirect() {
	status, header, body := this.sendRawPageErrorsRequest(
		"POST /api/participants HTTP/1.1\r\nHost: localhost:8080\r\nContent-Length: abc\r\n\r\n",
	)
	this.Equal(http.StatusBadRequest, status)
	this.Contains(header.Get("Content-Type"), "application/json")
	this.Empty(header.Get("Location"))
	this.Contains(body, `"message"`)
}

func (this *IntegrationSuite) TestUnhappyPath_UnparsableRequestHeadIsLoggedWithoutItsSecrets() {
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(previous)

	status, _, body := this.sendRawPageErrorsRequest("POST /set-password?token=secret-link-token HTTP/1.1\r\n" +
		"Host: localhost:8080\r\n" +
		"Cookie: bidirekt_session=secret-session-cookie\r\n" +
		"Authorization: Bearer secret-jwt\r\n" +
		"Content-Length: abc\r\n\r\n",
	)

	this.Equal(http.StatusBadRequest, status)
	this.Contains(logged.String(), "failed")
	for _, secret := range []string{"secret-link-token", "secret-session-cookie", "secret-jwt"} {
		this.NotContains(logged.String(), secret)
		this.NotContains(body, secret)
	}
}
