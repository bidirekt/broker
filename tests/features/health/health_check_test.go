package health_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bidirekt/broker/internal/features/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func healthServerAddress(t *testing.T, status int) string {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/health" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)

	return server.Listener.Addr().String()
}

func TestCheck_HealthRespondingOkIsNil(t *testing.T) {
	assert.NoError(t, health.Check(healthServerAddress(t, http.StatusOK)))
}

func TestCheck_AddressWithoutHostOrOnAllInterfacesTargetsLoopback(t *testing.T) {
	_, port, err := net.SplitHostPort(healthServerAddress(t, http.StatusOK))
	require.NoError(t, err)

	assert.NoError(t, health.Check(":"+port))
	assert.NoError(t, health.Check("0.0.0.0:"+port))
}

func TestCheck_HealthRespondingServiceUnavailableIsError(t *testing.T) {
	assert.ErrorContains(t, health.Check(healthServerAddress(t, http.StatusServiceUnavailable)), "responded 503")
}

func TestCheck_ClosedPortIsError(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddress := listener.Addr().String()
	require.NoError(t, listener.Close())

	assert.ErrorContains(t, health.Check(closedAddress), "connection refused")
}
