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

func (s *IntegrationSuite) SetupSuite() {
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
	s.Require().NoError(err)
	s.container = container

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)

	s.Require().NoError(os.Setenv("BIDIREKT_DATABASE_URL", connStr))

	s.Components, err = internal.Run()
	s.Require().NoError(err)
	s.Pool = s.Components.Pool
}

func (s *IntegrationSuite) TearDownSuite() {
	if s.Pool != nil {
		s.Pool.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *IntegrationSuite) SetupTest() {
	_, err := s.Pool.Exec(context.Background(),
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

	s.Require().NoError(err)
}

func (s *IntegrationSuite) post(path, body string) (status int, response string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	return resp.StatusCode, string(bytes)
}

type contractFragment struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

func (s *IntegrationSuite) publishBody(participant, version string, fragments ...contractFragment) string {
	body, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Version     string             `json:"version"`
		Contracts   []contractFragment `json:"contracts"`
	}{
		Participant: participant,
		Version:     version,
		Contracts:   fragments,
	})
	s.Require().NoError(err)

	return string(body)
}

func (s *IntegrationSuite) countRows(table string) int {
	var count int
	err := s.Pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s", table),
	).Scan(&count)
	s.Require().NoError(err)
	return count
}
