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

const refBodyJSON = `{
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
        "another":       { "type": "object", "properties": { "id": { "type": "string" } } },
        "somethingElse": { "type": "string" },
        "list": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "userId":        { "type": "string" },
              "another":       { "type": "object", "properties": { "id": { "type": "string" } } },
              "somethingElse": { "type": "string" },
              "list": {
                "type": "array",
                "items": { "type": "object", "properties": { "id": { "type": "string" } } }
              }
            }
          }
        }
      }
    }
  }
}`

const petListModuleYAML = `consumes:
  pets_api:
    rest:
      /pets:
        get:
          responses:
            "200": PetList
schemas:
  PetList:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      id: { type: string }
  Unused:
    type: object
    properties:
      note: { type: string }
`

const petSummaryModuleYAML = `consumes:
  pets_api:
    rest:
      /pets:
        get:
          responses:
            "200": PetSummary
schemas:
  PetSummary:
    type: object
    properties:
      name: { type: string }
`

const petCreationProviderYAML = `provides:
  rest:
    /pets:
      post:
        request: NewPet
        responses:
          "201": ~
schemas:
  NewPet:
    type: object
    properties:
      name: { type: string }
`

const undescribedBodiesOnlyYAML = `provides:
  rest:
    /files:
      post:
        request: ~
        responses:
          "204":
schemas:
  Orphan:
    type: string
`

type schemaFile struct {
	source string
	raw    string
}

func schemaModelsOf(t *testing.T, files ...schemaFile) model.Schemas {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		var document any
		require.NoError(t, yaml.Unmarshal([]byte(file.raw), &document))

		fragments = append(fragments, contract.Fragment{Source: file.source, Document: document})
	}

	return fragmentmapper.ToSchemaModels(fragmentmapper.ToDeclarations(fragments))
}

func TestToSchemaModels_PrimitiveBody_IsASchemaWithItsOwnFieldOnly(t *testing.T) {
	schemas := schemaModelsOf(t, schemaFile{"api.yaml", primitiveTopLevelJSON})

	assert.Equal(t, model.Schemas{
		"Pong": {"": {Type: "string"}},
	}, schemas)
}

func TestToSchemaModels_ArrayOfInlineObjects_KeepsTheItemsInTheSameSchema(t *testing.T) {
	schemas := schemaModelsOf(t, schemaFile{"api.yaml", arrayOfObjectsJSON})

	assert.Equal(t, model.Schemas{
		"PetList": {
			"":      {Type: "array"},
			"[]":    {Type: "object"},
			"[].id": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_BodyThatIsARef_KeepsTheRefAndConvertsTheTarget(t *testing.T) {
	schemas := schemaModelsOf(t, schemaFile{"api.yaml", refBodyJSON})

	assert.Equal(t, model.Schemas{
		"PetRef": {"": {Ref: "Pet"}},
		"Pet": {
			"":   {Type: "object"},
			"id": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_WideSchema_KeepsEveryDeclaredFieldOnce(t *testing.T) {
	schemas := schemaModelsOf(t, schemaFile{"api.yaml", wideShallowContractJSON})

	assert.Equal(t, model.Schemas{
		"UsersList": {
			"":        {Type: "object"},
			"users":   {Type: "array"},
			"users[]": {Ref: "User"},
		},
		"User": {
			"":                     {Type: "object"},
			"userId":               {Type: "string"},
			"another":              {Type: "object"},
			"another.id":           {Type: "string"},
			"somethingElse":        {Type: "string"},
			"list":                 {Type: "array"},
			"list[]":               {Type: "object"},
			"list[].userId":        {Type: "string"},
			"list[].another":       {Type: "object"},
			"list[].another.id":    {Type: "string"},
			"list[].somethingElse": {Type: "string"},
			"list[].list":          {Type: "array"},
			"list[].list[]":        {Type: "object"},
			"list[].list[].id":     {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_EveryDeclarationOfEveryModule_IsARoot(t *testing.T) {
	schemas := schemaModelsOf(t,
		schemaFile{"summary.yaml", petSummaryModuleYAML},
		schemaFile{"provider.yaml", petCreationProviderYAML},
		schemaFile{"list.yaml", petListModuleYAML},
	)

	assert.Equal(t, model.Schemas{
		"PetList": {
			"":   {Type: "array"},
			"[]": {Ref: "Pet"},
		},
		"Pet": {
			"":   {Type: "object"},
			"id": {Type: "string"},
		},
		"PetSummary": {
			"":     {Type: "object"},
			"name": {Type: "string"},
		},
		"NewPet": {
			"":     {Type: "object"},
			"name": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_UndescribedBodies_ConvertNoSchema(t *testing.T) {
	schemas := schemaModelsOf(t, schemaFile{"files.yaml", undescribedBodiesOnlyYAML})

	assert.Equal(t, model.Schemas{}, schemas)
}
