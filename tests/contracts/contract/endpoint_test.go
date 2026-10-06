package contract_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeEndpoint_TrimsTrailingSlash(t *testing.T) {
	assert.Equal(t, "/users", contract.NormalizeEndpoint("/users/"))
}

func TestNormalizeEndpoint_KeepsRoot(t *testing.T) {
	assert.Equal(t, "/", contract.NormalizeEndpoint("/"))
}

func TestNormalizeEndpoint_LeavesPlainEndpointAlone(t *testing.T) {
	assert.Equal(t, "/users", contract.NormalizeEndpoint("/users"))
}
