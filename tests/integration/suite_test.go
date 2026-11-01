package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	suitePublicURL         = "http://localhost:8080"
	suiteAdminUsername     = "admin"
	suiteAdminPassword     = "integration-admin-password"
	suiteSessionCookieName = "bidirekt_session"
	suiteWriterEmail       = "suite-writer@x.com"
	suiteWriterPassword    = "suite-writer-correct-horse"
)

var csrfTokenInPage = regexp.MustCompile(`"X-CSRF-Token": "([^"]+)"`)

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

type IntegrationSuite struct {
	suite.Suite

	container   *postgres.PostgresContainer
	Components  *components.Components
	Pool        *pgxpool.Pool
	writerToken string
	testCount   int
	clientIP    string
}

func (this *IntegrationSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18.6-alpine",
		postgres.WithDatabase("contracttests"),
		postgres.WithUsername("contracttests"),
		postgres.WithPassword("s3cr3t"),
		// An ICU default collation orders names differently from "C", so a
		// ListNames query that drops COLLATE "C" fails the SortedByteWise tests.
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_INITDB_ARGS": "--locale-provider=icu --icu-locale=en-US",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	this.Require().NoError(err)
	this.container = container

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	this.Require().NoError(err)

	this.Require().NoError(os.Setenv("BIDIREKT_DATABASE_URL", connStr))
	this.Require().NoError(os.Setenv("BIDIREKT_PUBLIC_URL", suitePublicURL))
	this.Require().NoError(os.Setenv("BIDIREKT_ADMIN_USERNAME", suiteAdminUsername))
	this.Require().NoError(os.Setenv("BIDIREKT_ADMIN_PASSWORD", suiteAdminPassword))
	this.Require().NoError(os.Setenv("BIDIREKT_TRUSTED_PROXIES", "0.0.0.0"))

	this.Components, err = internal.Run()
	this.Require().NoError(err)
	this.Pool = this.Components.Pool
}

func (this *IntegrationSuite) TearDownSuite() {
	if this.Pool != nil {
		this.Pool.Close()
	}
	if this.container != nil {
		_ = this.container.Terminate(context.Background())
	}
}

func (this *IntegrationSuite) SetupTest() {
	_, err := this.Pool.Exec(context.Background(),
		`TRUNCATE 
			compatibility_check_results,
			compatibility_checks,
			compatibility_verdicts,
			deployments,
			resource_bodies,
			field_versions,
			fields,
			schema_versions,
			schemas,
			resource_versions,
			resources,
			contract_versions,
			contracts,
			participants,
			environments,
			sessions,
			one_time_links,
			users
			RESTART IDENTITY CASCADE`,
	)

	this.Require().NoError(err)

	this.testCount++
	this.clientIP = fmt.Sprintf("10.%d.%d.%d", this.testCount>>16&255, this.testCount>>8&255, this.testCount&255)

	this.createActiveUser(suiteWriterEmail, model.PermissionWrite, suiteWriterPassword)
	this.writerToken = this.signInToApi(suiteWriterEmail, suiteWriterPassword)
}

func (this *IntegrationSuite) deleteSuiteWriter() {
	_, err := this.Pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, suiteWriterEmail)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) signInToApi(email, password string) (token string) {
	requestBody, err := json.Marshal(map[string]string{"email": email, "password": password})
	this.Require().NoError(err)

	status, _, body := this.postWithToken("/api/sign-in", "", string(requestBody))
	this.Require().Equal(http.StatusOK, status, body)

	var response struct {
		Token string `json:"token"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response.Token
}

func (this *IntegrationSuite) post(path, body string) (status int, response string) {
	status, _, response = this.postWithToken(path, this.writerToken, body)
	return status, response
}

func (this *IntegrationSuite) get(path string) (status int, response string) {
	status, _, response = this.getWithToken(path, this.writerToken)
	return status, response
}

func (this *IntegrationSuite) postWithToken(path, token, body string) (status int, header http.Header, response string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	return this.sendApiRequest(req, token)
}

func (this *IntegrationSuite) getWithToken(path, token string) (status int, header http.Header, response string) {
	req := httptest.NewRequest("GET", path, nil)

	return this.sendApiRequest(req, token)
}

func (this *IntegrationSuite) sendApiRequest(req *http.Request, token string) (status int, header http.Header, response string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Forwarded-For", this.clientIP)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

type contractFragment struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

func (this *IntegrationSuite) publishBody(participant, version string, fragments ...contractFragment) string {
	body, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Version     string             `json:"version"`
		Contracts   []contractFragment `json:"contracts"`
	}{
		Participant: participant,
		Version:     version,
		Contracts:   fragments,
	})
	this.Require().NoError(err)

	return string(body)
}

func (this *IntegrationSuite) countRows(table string) int {
	var count int
	err := this.Pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s", table),
	).Scan(&count)
	this.Require().NoError(err)
	return count
}

func (this *IntegrationSuite) getPage(path, sessionCookie string) (status int, header http.Header, body string) {
	req := httptest.NewRequest("GET", path, nil)

	return this.sendPageRequest(req, sessionCookie)
}

func (this *IntegrationSuite) postForm(path, sessionCookie, csrfToken string, form url.Values) (status int, header http.Header, body string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", suitePublicURL)
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	return this.sendPageRequest(req, sessionCookie)
}

func (this *IntegrationSuite) sendPageRequest(req *http.Request, sessionCookie string) (status int, header http.Header, body string) {
	req.Header.Set("Accept", "text/html")
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: suiteSessionCookieName, Value: sessionCookie})
	}
	req.Header.Set("X-Forwarded-For", this.clientIP)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, resp.Header, string(bytes)
}

func (this *IntegrationSuite) signInAsAdmin() (sessionCookie string) {
	return this.signIn(suiteAdminUsername, suiteAdminPassword)
}

func (this *IntegrationSuite) signInAsUser(email, password string) (sessionCookie string) {
	return this.signIn(email, password)
}

func (this *IntegrationSuite) signIn(identifier, password string) (sessionCookie string) {
	status, header, _ := this.postForm("/sign-in", "", "", url.Values{"identifier": {identifier}, "password": {password}})
	this.Require().Equal(http.StatusSeeOther, status)

	cookie, err := http.ParseSetCookie(header.Get("Set-Cookie"))
	this.Require().NoError(err)
	this.Require().Equal(suiteSessionCookieName, cookie.Name)

	return cookie.Value
}

func (this *IntegrationSuite) csrfTokenFrom(page string) string {
	match := csrfTokenInPage.FindStringSubmatch(page)
	this.Require().Len(match, 2, "page carries no CSRF token")

	return match[1]
}

func (this *IntegrationSuite) createActiveUser(email string, permission model.Permission, password string) (userID int64) {
	passwordHash, err := components.HashPassword(password)
	this.Require().NoError(err)

	err = this.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, permission, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, permission, passwordHash,
	).Scan(&userID)
	this.Require().NoError(err)

	return userID
}

func (this *IntegrationSuite) setUserState(userID int64, state model.UserState) {
	query := map[model.UserState]string{
		model.UserInvited:     `UPDATE users SET password_hash = NULL, active = true WHERE id = $1`,
		model.UserActive:      `UPDATE users SET active = true WHERE id = $1 AND password_hash IS NOT NULL`,
		model.UserDeactivated: `UPDATE users SET active = false WHERE id = $1`,
	}[state]

	tag, err := this.Pool.Exec(context.Background(), query, userID)
	this.Require().NoError(err)
	this.Require().EqualValues(1, tag.RowsAffected(), "user %d cannot be set %s", userID, state)
}
