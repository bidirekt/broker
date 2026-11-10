package fragmentmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
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

	return fragmentmapper.ToResourceModels(fragmentmapper.ToDeclarations(singleFragment(t, raw), protocol.NewRegistry([]protocol.Protocol{restcontracts.New()})))
}

func TestToResourceModels_Happy_MaterializesResources(t *testing.T) {
	resources := toResourceModels(t, happyContractJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	assert.Equal(t, model.Consumes, resource.Direction)
	assert.Equal(t, model.Output, resource.Body)
	assert.Equal(t, "pets_service", resource.ConsumedProvider.String)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"}, resource.Identity)
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
		switch r.Body {
		case model.Input:
			request = r
		case model.Output:
			response = r
		}
	}

	assert.Equal(t, model.Consumes, request.Direction)
	assert.Equal(t, model.Input, request.Body)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "post"}, request.Identity)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("NewPet")}}, request.Bodies)

	assert.Equal(t, model.Consumes, response.Direction)
	assert.Equal(t, model.Output, response.Body)
	assert.Equal(t, "201", response.Identity.(restmodel.Identity).Status)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}}, response.Bodies)
}

func TestToResourceModels_ProvidesSide_EmitsProvidedResource(t *testing.T) {
	resources := toResourceModels(t, provideRestResponseJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	assert.Equal(t, model.Provides, resource.Direction)
	assert.Equal(t, model.Output, resource.Body)
	assert.Empty(t, resource.ConsumedProvider)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"}, resource.Identity)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}}, resource.Bodies)
}
