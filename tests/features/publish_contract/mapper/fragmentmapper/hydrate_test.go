package fragmentmapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/features/publish_contract/contract"
	"github.com/bidirekt/broker/internal/features/publish_contract/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/goccy/go-yaml"
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

	assert.Contains(t, resource.Properties, "$")
	assert.Contains(t, resource.Properties, "$.id")
	assert.Contains(t, resource.Properties, "$.name")
	assert.Equal(t, "string", resource.Properties["$.id"].Type)
}

const postWithRequestBodyJSON = `{
  "consumes": {
    "pets_service": {
      "rest": {
        "/pets": {
          "post": {
            "request": "Pet",
            "responses": { "201": "Pet" }
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

const primitiveTopLevelJSON = `{
  "consumes": {
    "ping-service": {
      "rest": {
        "/ping": {
          "get": { "responses": { "200": "Pong" } }
        }
      }
    }
  },
  "schemas": {
    "Pong": { "type": "string" }
  }
}`

const arrayOfObjectsJSON = `{
  "consumes": {
    "pets_service": {
      "rest": {
        "/pets": {
          "get": { "responses": { "200": "PetList" } }
        }
      }
    }
  },
  "schemas": {
    "PetList": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string" }
        }
      }
    }
  }
}`

const refResolvesJSON = `{
  "consumes": {
    "pets_service": {
      "rest": {
        "/pets": {
          "get": { "responses": { "200": "PetRef" } }
        }
      }
    }
  },
  "schemas": {
    "PetRef": { "ref": "Pet" },
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
	assert.Contains(t, request.Properties, "$.id")

	assert.Equal(t, model.Consumes, response.Direction)
	assert.Equal(t, model.RestResponse, response.Interaction)
	assert.Equal(t, "201", response.ResponseStatusCode.String)
	assert.Contains(t, response.Properties, "$.name")
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
	assert.Contains(t, resource.Properties, "$.id")
}

func TestToResourceModels_PrimitiveTopLevel_EmitsRootPrimitive(t *testing.T) {
	resources := toResourceModels(t, primitiveTopLevelJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	require.Contains(t, resource.Properties, "$")
	assert.Equal(t, "string", resource.Properties["$"].Type)
	assert.Len(t, resource.Properties, 1)
}

func TestToResourceModels_ArrayOfObjects_WalksItemsViaSchemaPointer(t *testing.T) {
	resources := toResourceModels(t, arrayOfObjectsJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	require.Contains(t, resource.Properties, "$")
	require.Contains(t, resource.Properties, "$[]")
	require.Contains(t, resource.Properties, "$[].id")
	assert.Equal(t, "array", resource.Properties["$"].Type)
	assert.Equal(t, "object", resource.Properties["$[]"].Type)
	assert.Equal(t, "string", resource.Properties["$[].id"].Type)
}

func TestToResourceModels_RefResolves_SubstitutesReferencedSchema(t *testing.T) {
	resources := toResourceModels(t, refResolvesJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	require.Contains(t, resource.Properties, "$")
	require.Contains(t, resource.Properties, "$.id")
	assert.Equal(t, "object", resource.Properties["$"].Type)
	assert.Equal(t, "string", resource.Properties["$.id"].Type)
}

const wideShallowContractJSON = `{
  "provides": {
    "rest": {
      "/users": {
        "get": {
          "responses": { "200": "UsersList" }
        }
      }
    }
  },
  "schemas": {
    "UsersList": {
      "type": "object",
      "properties": {
        "users": {
          "type": "array",
          "items": { "ref": "User" }
        }
      }
    },
    "User": {
      "type": "object",
      "properties": {
        "userId":        { "type": "string" },
        "another":       { "type": "object" },
        "somethingElse": { "type": "string" },
        "list": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "userId":        { "type": "string" },
              "another":       { "type": "object" },
              "somethingElse": { "type": "string" },
              "list": {
                "type": "array",
                "items": { "type": "object" }
              }
            }
          }
        }
      }
    }
  }
}`

func TestToResourceModels_WideShallowSchema_MaterializesEveryBranch(t *testing.T) {
	resources := toResourceModels(t, wideShallowContractJSON)

	require.Len(t, resources, 1)
	resource := resources[0]

	require.Contains(t, resource.Properties, "$.users[].list[].list[]")
	assert.Equal(t, "array", resource.Properties["$.users[].list[].list"].Type)
	assert.Equal(t, "string", resource.Properties["$.users[].list[].userId"].Type)
}
