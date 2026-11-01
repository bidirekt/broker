package components_test

import (
	"bytes"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const databasePassword = "s3cr3t"

const adminPassword = "a-valid-admin-password"

func setValidAuthSettings(t *testing.T) {
	t.Setenv("BIDIREKT_PUBLIC_URL", "https://broker.example.com")
	t.Setenv("BIDIREKT_ADMIN_USERNAME", "admin")
	t.Setenv("BIDIREKT_ADMIN_PASSWORD", adminPassword)
}

const packetDroppingAddress = "10.255.255.1:5432"

func closedPortAddress(t *testing.T) string {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	return address
}

func databaseURL(address string) string {
	return "postgres://broker:" + databasePassword + "@" + address + "/broker?sslmode=disable"
}

func TestNew_WithoutDatabaseURLFailsAsRequired(t *testing.T) {
	setValidAuthSettings(t)
	t.Setenv("BIDIREKT_DATABASE_URL", "")

	_, err := components.New()

	assert.EqualError(t, err, "BIDIREKT_DATABASE_URL is required")
}

func TestNew_MalformedDatabaseURLFailsWithoutConnecting(t *testing.T) {
	setValidAuthSettings(t)
	t.Setenv("BIDIREKT_DATABASE_URL", "postgres://broker:"+databasePassword+"@localhost:notaport/broker")

	_, err := components.New()

	require.ErrorContains(t, err, "invalid BIDIREKT_DATABASE_URL")
	assert.NotContains(t, err.Error(), databasePassword)
}

func TestNew_ConnectRetriesThatAreNotAPositiveIntegerAreRejected(t *testing.T) {
	for _, retries := range []string{"0", "-1", "abc", "1.5"} {
		t.Run(retries, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(closedPortAddress(t)))
			t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", retries)

			_, err := components.New()

			assert.EqualError(t, err, "BIDIREKT_DATABASE_CONNECT_RETRIES must be a positive integer")
		})
	}
}

func TestNew_ClosedPortWithOneAttemptReturnsError(t *testing.T) {
	setValidAuthSettings(t)
	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(closedPortAddress(t)))
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "1")

	_, err := components.New()

	assert.ErrorContains(t, err, "database not ready")
}

func TestNew_LogsEveryAttemptAndPausesBetweenThem(t *testing.T) {
	setValidAuthSettings(t)
	var logged bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(closedPortAddress(t)))
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "2")

	started := time.Now()
	_, err := components.New()

	require.Error(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 2*time.Second)
	assert.Contains(t, logged.String(), "database not ready (attempt 1/2): ")
	assert.Contains(t, logged.String(), "database not ready (attempt 2/2): ")
}

func TestNew_PacketDroppingHostGivesUpWithinTheAttemptTimeout(t *testing.T) {
	setValidAuthSettings(t)
	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(packetDroppingAddress))
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "1")

	started := time.Now()
	_, err := components.New()

	require.Error(t, err)
	assert.Less(t, time.Since(started), 7*time.Second)
}

func TestNew_ConnectTimeoutInTheURLBoundsTheAttempt(t *testing.T) {
	setValidAuthSettings(t)
	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(packetDroppingAddress)+"&connect_timeout=1")
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "1")

	started := time.Now()
	_, err := components.New()

	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}

func TestNew_RefusesAPublicURLThatIsNotAnAllowedOrigin(t *testing.T) {
	const originRule = "BIDIREKT_PUBLIC_URL must be an origin (scheme://host[:port]) without a path, query, fragment or userinfo"
	const schemeRule = "BIDIREKT_PUBLIC_URL must use https, or http on localhost or 127.0.0.1"

	for _, refusal := range []struct {
		publicURL string
		message   string
	}{
		{"", "BIDIREKT_PUBLIC_URL is required"},
		{"https://broker.example.com/broker", originRule},
		{"https://broker.example.com/", originRule},
		{"https://broker.example.com?team=a", originRule},
		{"https://broker.example.com?", originRule},
		{"https://broker.example.com#top", originRule},
		{"https://user:pass@broker.example.com", originRule},
		{"https://broker.example.com:", originRule},
		{"https://", originRule},
		{"https://Broker.Example.com", "BIDIREKT_PUBLIC_URL must have a lower-case host"},
		{"https://broker.example.com:443", "BIDIREKT_PUBLIC_URL must not write out the default port"},
		{"http://localhost:80", "BIDIREKT_PUBLIC_URL must not write out the default port"},
		{"https://broker.example.com:08443", "BIDIREKT_PUBLIC_URL must have a port from 1 to 65535 without leading zeros"},
		{"https://broker.example.com:0", "BIDIREKT_PUBLIC_URL must have a port from 1 to 65535 without leading zeros"},
		{"http://broker.example.com", schemeRule},
		{"http://localhost.evil.com", schemeRule},
		{"http://127.0.0.2:8080", schemeRule},
		{"http://[::1]:8080", schemeRule},
		{"HTTPS://broker.example.com", schemeRule},
		{"ftp://broker.example.com", schemeRule},
		{"broker.example.com", schemeRule},
	} {
		t.Run(refusal.publicURL, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_PUBLIC_URL", refusal.publicURL)
			t.Setenv("BIDIREKT_DATABASE_URL", "")

			_, err := components.New()

			assert.EqualError(t, err, refusal.message)
		})
	}
}

func TestNew_RefusesAMissingAdminUsernameOrOneWithAnAt(t *testing.T) {
	for _, refusal := range []struct {
		username string
		message  string
	}{
		{"", "BIDIREKT_ADMIN_USERNAME is required"},
		{"admin@example.com", "BIDIREKT_ADMIN_USERNAME must not contain @"},
	} {
		t.Run(refusal.username, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_ADMIN_USERNAME", refusal.username)
			t.Setenv("BIDIREKT_DATABASE_URL", "")

			_, err := components.New()

			assert.EqualError(t, err, refusal.message)
		})
	}
}

func TestNew_RefusesAMissingAdminPasswordOrOneOutsideFifteenToOneHundredTwentyEightCharacters(t *testing.T) {
	for _, refusal := range []struct {
		password string
		message  string
	}{
		{"", "BIDIREKT_ADMIN_PASSWORD is required"},
		{"fourteen-chars", "BIDIREKT_ADMIN_PASSWORD must have 15 to 128 characters"},
		{strings.Repeat("p", 129), "BIDIREKT_ADMIN_PASSWORD must have 15 to 128 characters"},
	} {
		t.Run(strconv.Itoa(len(refusal.password)), func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_ADMIN_PASSWORD", refusal.password)
			t.Setenv("BIDIREKT_DATABASE_URL", "")

			_, err := components.New()

			assert.EqualError(t, err, refusal.message)
		})
	}
}

func TestNew_AcceptedSettingsReachTheDatabaseChecks(t *testing.T) {
	for _, publicURL := range []string{"https://broker.example.com", "https://broker.example.com:8443", "http://localhost:8080", "http://127.0.0.1:8080", "http://localhost"} {
		t.Run(publicURL, func(t *testing.T) {
			setValidAuthSettings(t)
			t.Setenv("BIDIREKT_PUBLIC_URL", publicURL)
			t.Setenv("BIDIREKT_DATABASE_URL", "")

			_, err := components.New()

			assert.EqualError(t, err, "BIDIREKT_DATABASE_URL is required")
		})
	}
}
