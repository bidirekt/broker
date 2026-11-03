package database_test

import (
	"bytes"
	"log"
	"net"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/components/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const databasePassword = "s3cr3t"

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

func TestOpen_WithoutDatabaseURLFailsAsRequired(t *testing.T) {
	t.Setenv("BIDIREKT_DATABASE_URL", "")

	_, err := database.Open()

	assert.EqualError(t, err, "BIDIREKT_DATABASE_URL is required")
}

func TestOpen_MalformedDatabaseURLFailsWithoutConnecting(t *testing.T) {
	t.Setenv("BIDIREKT_DATABASE_URL", "postgres://broker:"+databasePassword+"@localhost:notaport/broker")

	_, err := database.Open()

	require.ErrorContains(t, err, "invalid BIDIREKT_DATABASE_URL")
	assert.NotContains(t, err.Error(), databasePassword)
}

func TestOpen_ConnectRetriesThatAreNotAPositiveIntegerAreRejected(t *testing.T) {
	for _, retries := range []string{"0", "-1", "abc", "1.5"} {
		t.Run(retries, func(t *testing.T) {
			t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(closedPortAddress(t)))
			t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", retries)

			_, err := database.Open()

			assert.EqualError(t, err, "BIDIREKT_DATABASE_CONNECT_RETRIES must be a positive integer")
		})
	}
}

func TestOpen_ClosedPortWithOneAttemptReturnsError(t *testing.T) {
	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(closedPortAddress(t)))
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "1")

	_, err := database.Open()

	assert.ErrorContains(t, err, "database not ready")
}

func TestOpen_LogsEveryAttemptAndPausesBetweenThem(t *testing.T) {
	var logged bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(closedPortAddress(t)))
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "2")

	started := time.Now()
	_, err := database.Open()

	require.Error(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 2*time.Second)
	assert.Contains(t, logged.String(), "database not ready (attempt 1/2): ")
	assert.Contains(t, logged.String(), "database not ready (attempt 2/2): ")
}

func TestOpen_PacketDroppingHostGivesUpWithinTheAttemptTimeout(t *testing.T) {
	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(packetDroppingAddress))
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "1")

	started := time.Now()
	_, err := database.Open()

	require.Error(t, err)
	assert.Less(t, time.Since(started), 7*time.Second)
}

func TestOpen_ConnectTimeoutInTheURLBoundsTheAttempt(t *testing.T) {
	t.Setenv("BIDIREKT_DATABASE_URL", databaseURL(packetDroppingAddress)+"&connect_timeout=1")
	t.Setenv("BIDIREKT_DATABASE_CONNECT_RETRIES", "1")

	started := time.Now()
	_, err := database.Open()

	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}
