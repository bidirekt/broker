package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

type IntegrationSuite struct {
	suite.Suite

	container  *postgres.PostgresContainer
	Components *components.Components
	Pool       *pgxpool.Pool
}

func (this *IntegrationSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18.6-alpine",
		postgres.WithDatabase("contracttests"),
		postgres.WithUsername("contracttests"),
		postgres.WithPassword("s3cr3t"),
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
			property_versions,
			resource_versions,
			properties,
			resources,
			contract_versions,
			contracts,
			participants,
			environments
			RESTART IDENTITY CASCADE`,
	)

	this.Require().NoError(err)
}

func (this *IntegrationSuite) post(path, body string) (status int, response string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, string(bytes)
}

func (this *IntegrationSuite) get(path string) (status int, response string) {
	req := httptest.NewRequest("GET", path, nil)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)

	return resp.StatusCode, string(bytes)
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
