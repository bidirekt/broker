package fragmentmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func endpointProvidesJSON(endpoint string) string {
	return `{
  "provides": {
    "rest": {
      "` + endpoint + `": {
        "get": { "responses": { "200": "Thing" } }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": { "id": { "type": "string" } }
    }
  }
}`
}

func singleEndpointResource(t *testing.T, raw string) model.UploadedResource {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(raw), &document))

	declarations := fragmentmapper.ToDeclarations([]contract.Fragment{{Source: "things.yaml", Document: document}})
	resources := fragmentmapper.ToResourceModels(declarations)
	require.Len(t, resources, 1)

	return resources[0]
}

func TestEndpointRules_Accepts(t *testing.T) {
	for _, endpoint := range []string{"/users", "/users/*", "/users/*/orders/*", "/"} {
		t.Run(endpoint, func(t *testing.T) {
			resource := singleEndpointResource(t, endpointProvidesJSON(endpoint))

			assert.Equal(t, endpoint, resource.Endpoint)
		})
	}
}

func TestEndpointRules_TrailingSlash_NormalizesToSameResource(t *testing.T) {
	slashed := singleEndpointResource(t, endpointProvidesJSON("/users/"))
	plain := singleEndpointResource(t, endpointProvidesJSON("/users"))

	assert.Equal(t, "/users", slashed.Endpoint)
	assert.Equal(t, plain.ProviderHash(), slashed.ProviderHash())
}
