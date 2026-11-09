package rest_test

import (
	"testing"

	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/stretchr/testify/assert"
)

func TestIdentity_Response_NamesItsStatus(t *testing.T) {
	identity := restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"}

	assert.Equal(t, []string{"/pets", "get", "200"}, identity.HashParts())
	assert.Equal(t, "200", identity.Interaction())
	assert.Equal(t, "GET /pets 200", identity.Describe())
	assert.Equal(t, map[string]string{"endpoint": "/pets", "method": "get", "interaction": "200"}, identity.Location())
}

func TestIdentity_Request_IsNamedRequest(t *testing.T) {
	identity := restmodel.Identity{Endpoint: "/pets", Method: "post"}

	assert.Equal(t, []string{"/pets", "post"}, identity.HashParts())
	assert.Equal(t, "request", identity.Interaction())
	assert.Equal(t, "POST /pets request", identity.Describe())
	assert.Equal(t, map[string]string{"endpoint": "/pets", "method": "post", "interaction": "request"}, identity.Location())
}
