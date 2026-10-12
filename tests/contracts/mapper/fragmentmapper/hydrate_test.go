package fragmentmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/goccy/go-yaml"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const happyContractJSON = `{
  "consumes": {
    "pets_service": {
      "rest": {
        "/pets": {
          "get": {
            "responses": {
              "200": "Pet"
            }
          }
        }
      }
    }
  },
  "schemas": {
    "Pet": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

func singleFragment(t *testing.T, raw string) []contract.Fragment {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(raw), &document))

	return []contract.Fragment{{Source: "api.yaml", Document: document}}
}

func toResourceModels(t *testing.T, raw string) []model.UploadedResource {
	t.Helper()

	return fragmentmapper.ToResourceModels(fragmentmapper.ToDeclarations(singleFragment(t, raw)))
}

func TestToResourceModels_Happy_MaterializesResources(t *testing.T) {
	resources := toResourceModels(t, happyContractJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	assert.Equal(t, model.Consumes, resource.Direction)
	assert.Equal(t, model.RestResponse, resource.Interaction)
	assert.Equal(t, "pets_service", resource.ConsumedProvider.String)
	assert.Equal(t, "/pets", resource.Endpoint)
	assert.Equal(t, "get", resource.Method)
	assert.Equal(t, "200", resource.ResponseStatusCode.String)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}}, resource.Bodies)
}

const postWithRequestBodyJSON = `{
  "consumes": {
    "pets_service": {
      "rest": {
        "/pets": {
          "post": {
            "request": "NewPet",
            "responses": { "201": "Pet" }
          }
        }
      }
    }
  },
  "schemas": {
    "NewPet": {
      "type": "object",
      "properties": {
        "name": { "type": "string" }
      }
    },
    "Pet": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const provideRestResponseJSON = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": {
          "responses": { "200": "Pet" }
        }
      }
    }
  },
  "schemas": {
    "Pet": {
      "type": "object",
      "properties": { "id": { "type": "string" } }
    }
  }
}`

func TestToResourceModels_PostWithRequestBody_EmitsRequestAndResponses(t *testing.T) {
	resources := toResourceModels(t, postWithRequestBodyJSON)

	require.Len(t, resources, 2)

	var request, response model.UploadedResource
	for _, r := range resources {
		switch r.Interaction {
		case model.RestRequest:
			request = r
		case model.RestResponse:
			response = r
		}
	}

	assert.Equal(t, model.Consumes, request.Direction)
	assert.Equal(t, model.RestRequest, request.Interaction)
	assert.Equal(t, "/pets", request.Endpoint)
	assert.Equal(t, "post", request.Method)
	assert.Empty(t, request.ResponseStatusCode)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("NewPet")}}, request.Bodies)

	assert.Equal(t, model.Consumes, response.Direction)
	assert.Equal(t, model.RestResponse, response.Interaction)
	assert.Equal(t, "201", response.ResponseStatusCode.String)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}}, response.Bodies)
}

func TestToResourceModels_ProvidesSide_EmitsProvidedResource(t *testing.T) {
	resources := toResourceModels(t, provideRestResponseJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	assert.Equal(t, model.Provides, resource.Direction)
	assert.Equal(t, model.RestResponse, resource.Interaction)
	assert.Empty(t, resource.ConsumedProvider)
	assert.Equal(t, "/pets", resource.Endpoint)
	assert.Equal(t, "get", resource.Method)
	assert.Equal(t, "200", resource.ResponseStatusCode.String)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}}, resource.Bodies)
}
