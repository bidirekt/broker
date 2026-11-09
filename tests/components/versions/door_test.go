package versions_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bidirekt/broker/internal/components/versions"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	doorTestCliVersionHeader = "Bidirekt-Cli-Version"
	doorTestPassedBody       = "passed"
)

func doorTestSend(t *testing.T, path, cliVersion string) (status int, body string) {
	server := fiber.New()
	server.Use(versions.Door)
	server.Get(path, func(ctx fiber.Ctx) error { return ctx.SendString(doorTestPassedBody) })

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cliVersion != "" {
		req.Header.Set(doorTestCliVersionHeader, cliVersion)
	}

	resp, err := server.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(bytes)
}

func TestBroker_IsDevWhenTheBuildDoesNotSetIt(t *testing.T) {
	assert.Equal(t, "dev", versions.Broker())
}

func TestMinimumCli_IsZeroDotOne(t *testing.T) {
	assert.Equal(t, "0.1.0", versions.MinimumCli)
}

func TestDoor_RefusesACliBelowTheMinimumOnTheAPI(t *testing.T) {
	status, body := doorTestSend(t, "/api/participants", "0.0.9")

	assert.Equal(t, http.StatusUpgradeRequired, status)
	assert.JSONEq(t, `{"message":"bidirekt 0.0.9 doesn't work with this broker (dev); upgrade bidirekt to 0.1.0 or later"}`, body)
}

func TestDoor_PassesACliAtOrAboveTheMinimum(t *testing.T) {
	for _, cliVersion := range []string{"0.1.0", "0.1", "0.2.0-rc.1", "1.0.0"} {
		status, body := doorTestSend(t, "/api/participants", cliVersion)

		assert.Equal(t, http.StatusOK, status, cliVersion)
		assert.Equal(t, doorTestPassedBody, body, cliVersion)
	}
}

func TestDoor_PassesARequestWithoutAVersion(t *testing.T) {
	for _, cliVersion := range []string{"", "dev", "v0.0.9", "0.0.9.1", "not-a-version"} {
		status, body := doorTestSend(t, "/api/participants", cliVersion)

		assert.Equal(t, http.StatusOK, status, cliVersion)
		assert.Equal(t, doorTestPassedBody, body, cliVersion)
	}
}

func TestDoor_DoesNotCheckPages(t *testing.T) {
	status, body := doorTestSend(t, "/participants", "0.0.9")

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, doorTestPassedBody, body)
}
