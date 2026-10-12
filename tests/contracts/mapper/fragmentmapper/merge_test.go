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

const listModuleJSON = `{
  "consumes": {
    "payments": {
      "rest": {
        "/invoices": {
          "get": { "responses": { "200": "InvoiceList" } }
        }
      }
    }
  },
  "schemas": {
    "InvoiceList": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const detailModuleJSON = `{
  "consumes": {
    "payments": {
      "rest": {
        "/invoices": {
          "get": { "responses": { "200": "InvoiceDetail" } }
        }
      }
    }
  },
  "schemas": {
    "InvoiceDetail": {
      "type": "object",
      "properties": {
        "id":         { "type": "string" },
        "created_at": { "type": "string" }
      }
    }
  }
}`

const createModuleJSON = `{
  "consumes": {
    "payments": {
      "rest": {
        "/invoices": {
          "post": { "request": "InvoiceCreate", "responses": { "201": "InvoiceCreate" } }
        }
      }
    }
  },
  "schemas": {
    "InvoiceCreate": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const importModuleJSON = `{
  "consumes": {
    "payments": {
      "rest": {
        "/invoices": {
          "post": { "request": "InvoiceImport", "responses": { "201": "InvoiceImport" } }
        }
      }
    }
  },
  "schemas": {
    "InvoiceImport": {
      "type": "object",
      "properties": {
        "id":    { "type": "string" },
        "email": { "type": "string" }
      }
    }
  }
}`

const providedPetsJSON = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": { "responses": { "200": "Pet" } }
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

const providedPetsDetailJSON = `{
  "provides": {
    "rest": {
      "/pets": {
        "get": { "responses": { "200": "PetDetail" } }
      }
    }
  },
  "schemas": {
    "PetDetail": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "name": { "type": "string" }
      }
    }
  }
}`

const groupNameYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: GroupName
          responses:
            "201": GroupName
schemas:
  GroupName:
    type: object
    properties:
      name: { type: string }
`

const groupUndescribedBodyYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: ~
          responses:
            "201": ~
`

type mergedFile struct {
	source string
	raw    string
}

type fileOrder struct {
	name  string
	files []mergedFile
}

func bothFileOrders(first, second mergedFile) []fileOrder {
	return []fileOrder{
		{"given order", []mergedFile{first, second}},
		{"reversed order", []mergedFile{second, first}},
	}
}

func mergeDeclarations(t *testing.T, files ...mergedFile) fragmentmapper.Declarations {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		var document any
		require.NoError(t, yaml.Unmarshal([]byte(file.raw), &document))

		fragments = append(fragments, contract.Fragment{Source: file.source, Document: document})
	}

	return fragmentmapper.ToDeclarations(fragments)
}

func mergeResources(t *testing.T, files ...mergedFile) []model.UploadedResource {
	t.Helper()

	return fragmentmapper.ToResourceModels(mergeDeclarations(t, files...))
}

func mergeContract(t *testing.T, files ...mergedFile) *model.UploadedContract {
	t.Helper()

	declarations := mergeDeclarations(t, files...)

	uploaded := model.NewUploadedContract(0, "front_app", "1", "")
	uploaded.Schemas = fragmentmapper.ToSchemaModels(declarations)

	for _, resource := range fragmentmapper.ToResourceModels(declarations) {
		require.NoError(t, uploaded.AddResource(&resource))
	}

	return uploaded
}

func mergedResource(t *testing.T, resources []model.UploadedResource, interaction model.Interaction) model.UploadedResource {
	t.Helper()

	for _, resource := range resources {
		if resource.Interaction == interaction {
			return resource
		}
	}

	t.Fatalf("no %v resource was built", interaction)

	return model.UploadedResource{}
}

func TestMerge_TwoConsumerModulesReadingOneResponse_OneResourceWithABodyPerModuleInSourceOrder(t *testing.T) {
	for _, order := range bothFileOrders(mergedFile{"list.yaml", listModuleJSON}, mergedFile{"detail.yaml", detailModuleJSON}) {
		t.Run(order.name, func(t *testing.T) {
			resources := mergeResources(t, order.files...)

			require.Len(t, resources, 1)

			resource := mergedResource(t, resources, model.RestResponse)
			assert.Equal(t, "/invoices", resource.Endpoint)
			assert.Equal(t, "200", resource.ResponseStatusCode.String)
			assert.Equal(t, []model.ResourceBody{
				{Source: "detail.yaml", SchemaName: null.StringFrom("InvoiceDetail")},
				{Source: "list.yaml", SchemaName: null.StringFrom("InvoiceList")},
			}, resource.Bodies)
		})
	}
}

func TestMerge_TwoConsumerModulesSendingOneRequest_OneResourceWithABodyPerModuleInSourceOrder(t *testing.T) {
	for _, order := range bothFileOrders(mergedFile{"import.yaml", importModuleJSON}, mergedFile{"create.yaml", createModuleJSON}) {
		t.Run(order.name, func(t *testing.T) {
			resources := mergeResources(t, order.files...)

			require.Len(t, resources, 2)

			request := mergedResource(t, resources, model.RestRequest)
			assert.Equal(t, []model.ResourceBody{
				{Source: "create.yaml", SchemaName: null.StringFrom("InvoiceCreate")},
				{Source: "import.yaml", SchemaName: null.StringFrom("InvoiceImport")},
			}, request.Bodies)
		})
	}
}

func TestMerge_UndescribedBodyInOneModule_IsABodyWithoutSchema(t *testing.T) {
	for _, order := range bothFileOrders(mergedFile{"a.yaml", groupUndescribedBodyYAML}, mergedFile{"b.yaml", groupNameYAML}) {
		t.Run(order.name, func(t *testing.T) {
			bodies := []model.ResourceBody{
				{Source: "a.yaml"},
				{Source: "b.yaml", SchemaName: null.StringFrom("GroupName")},
			}

			assert.Equal(t, []model.UploadedResource{
				*model.NewRestRequestConsumer("api", "/api/groups", "post", bodies),
				*model.NewRestResponseConsumer("api", "/api/groups", "post", "201", bodies),
			}, mergeResources(t, order.files...))
		})
	}
}

func TestMerge_UndescribedBodyInEveryModule_BuildsResourcesWithBodiesWithoutSchema(t *testing.T) {
	resources := mergeResources(t,
		mergedFile{"a.yaml", groupUndescribedBodyYAML},
		mergedFile{"b.yaml", groupUndescribedBodyYAML},
	)

	bodies := []model.ResourceBody{{Source: "a.yaml"}, {Source: "b.yaml"}}

	assert.Equal(t, []model.UploadedResource{
		*model.NewRestRequestConsumer("api", "/api/groups", "post", bodies),
		*model.NewRestResponseConsumer("api", "/api/groups", "post", "201", bodies),
	}, resources)
}

func TestMerge_IdenticalDeclarationInTwoFiles_BuildsOneResourceWithBothBodies(t *testing.T) {
	resources := mergeResources(t,
		mergedFile{"a.yaml", listModuleJSON},
		mergedFile{"b.yaml", listModuleJSON},
	)

	require.Len(t, resources, 1)
	assert.Equal(t, []model.ResourceBody{
		{Source: "a.yaml", SchemaName: null.StringFrom("InvoiceList")},
		{Source: "b.yaml", SchemaName: null.StringFrom("InvoiceList")},
	}, resources[0].Bodies)
}

func TestMerge_FragmentOrderReversed_ProducesTheSameContract(t *testing.T) {
	forward := mergeContract(t,
		mergedFile{"list.yaml", listModuleJSON},
		mergedFile{"detail.yaml", detailModuleJSON},
		mergedFile{"create.yaml", createModuleJSON},
		mergedFile{"import.yaml", importModuleJSON},
	)
	backward := mergeContract(t,
		mergedFile{"import.yaml", importModuleJSON},
		mergedFile{"create.yaml", createModuleJSON},
		mergedFile{"detail.yaml", detailModuleJSON},
		mergedFile{"list.yaml", listModuleJSON},
	)

	assert.Len(t, forward.Resources, 3)
	assert.Equal(t, forward.Resources, backward.Resources)
	assert.Equal(t, forward.Schemas, backward.Schemas)
	assert.Equal(t, forward.Checksum(), backward.Checksum())
}

func TestMerge_ProvidedResourceDeclaredTwice_FirstSourceWins(t *testing.T) {
	resources := mergeResources(t,
		mergedFile{"b.yaml", providedPetsDetailJSON},
		mergedFile{"a.yaml", providedPetsJSON},
	)

	require.Len(t, resources, 1)
	assert.Equal(t, []model.ResourceBody{{Source: "a.yaml", SchemaName: null.StringFrom("Pet")}}, resources[0].Bodies)
}
