package descriptor_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func contractDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestContract_ContratoValidoNaoTemViolacao(t *testing.T) {
	source := `provides:
  rest:
    "/todos":
      get:
        responses:
          "200": Todos
      post:
        request: CreateTodoRequest
        responses:
          "200": Todo
    "/todos/*":
      get:
        responses:
          200: Todo
      put:
        request: UpdateTodoRequest
        responses:
          200: Todo
      delete:
        responses:
          200: Message
consumes:
  billing_api:
    rest:
      "/invoices":
        get:
          responses:
            "200": Invoice
schemas:
  Todo:
    type: object
    description: um todo
    properties:
      todoId:
        type: integer
      title:
        type: string
        optional: true
      tags:
        type: array
        items:
          type: string
  Todos:
    type: array
    items:
      ref: Todo
  Invoice:
    type: object
    properties:
      total:
        type: float
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestContract_CincoErrosNumPasseSo(t *testing.T) {
	source := `provides:
  rest:
    /pets:
      patch:
        responses:
          200: Pet
    "/users/{id}":
      get:
        responses:
          200: Pet
consumes:
  billing:
    rest:
      /invoices:
        get:
          responses:
            600: Invoice
schemas:
  Pet:
    type: number
  Tags:
    type: array
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 5)

	for _, found := range violations {
		assert.Equal(t, "api.yaml", found.Source)
	}

	assert.Equal(t, "status.out_of_range", violations[0].ErrorCode)
	assert.Equal(t, "consumes;billing;rest;/invoices;get;responses;600", violations[0].Path)
	assert.Equal(t, "600", violations[0].Details["key"])

	assert.Equal(t, "key.unknown", violations[1].ErrorCode)
	assert.Equal(t, "provides;rest;/pets;patch", violations[1].Path)
	assert.Equal(t, map[string]string{"key": "patch"}, violations[1].Details)

	assert.Equal(t, "endpoint.syntax", violations[2].ErrorCode)
	assert.Equal(t, "provides;rest;/users/{id}", violations[2].Path)
	assert.Equal(t, "/users/{id}", violations[2].Details["key"])

	assert.Equal(t, "schema.invalid_type", violations[3].ErrorCode)
	assert.Equal(t, "schemas;Pet;type", violations[3].Path)
	assert.Equal(t, "number", violations[3].Details["value"])

	assert.Equal(t, "schema.array_without_items", violations[4].ErrorCode)
	assert.Equal(t, "schemas;Tags", violations[4].Path)
}

func TestContract_MessageViraChaveDesconhecida(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "provides:\n  message:\n    saudacao: ola\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "key.unknown", violations[0].ErrorCode)
	assert.Equal(t, "provides;message", violations[0].Path)
}

func TestContract_SchemaVazioEhTipoInvalido(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "schemas:\n  Pet: {}\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet", violations[0].Path)
	assert.Equal(t, "", violations[0].Details["value"])
	assert.Equal(t, "object, array, string, integer, float, boolean", violations[0].Details["allowed"])
}

func TestContract_SchemaNuloEhTipoInvalido(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "schemas:\n  Pet:\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet", violations[0].Path)
}

func TestContract_OptionalNaoBooleanoEhKind(t *testing.T) {
	source := "schemas:\n  Pet:\n    type: object\n    optional: talvez\n"

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet;optional", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "boolean", "got": "string"}, violations[0].Details)
}

func TestContract_DocumentoNuloNaoViola(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, nil, "api.yaml")

	assert.Empty(t, violations)
}

func TestContract_RaizQueNaoEhMapaEhKind(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "- a\n- b\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "mapping", "got": "sequence"}, violations[0].Details)
}

func TestContract_JsonDecodificadoSegueOMesmoCaminho(t *testing.T) {
	source := `{"provides": {"rest": {"/pets": {"get": {"responses": {"200": "Pet", "600": "Nope"}}}}}}`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "status.out_of_range", violations[0].ErrorCode)
	assert.Equal(t, "provides;rest;/pets;get;responses;600", violations[0].Path)
	assert.Equal(t, "api.yaml", violations[0].Source)
}

func TestContract_EndpointComParametroReprovaEmProvidesEConsumes(t *testing.T) {
	source := `provides:
  rest:
    "/users/{userId}":
      get:
        responses:
          200: User
consumes:
  users_api:
    rest:
      "/users/{userId}":
        get:
          responses:
            200: User
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 2)

	assert.Equal(t, "endpoint.syntax", violations[0].ErrorCode)
	assert.Equal(t, "consumes;users_api;rest;/users/{userId}", violations[0].Path)
	assert.Equal(t, "/users/{userId}", violations[0].Details["key"])

	assert.Equal(t, "endpoint.syntax", violations[1].ErrorCode)
	assert.Equal(t, "provides;rest;/users/{userId}", violations[1].Path)
	assert.Equal(t, "/users/{userId}", violations[1].Details["key"])
}

func TestContract_NomeDeServicoInvalidoSuprimeADescida(t *testing.T) {
	source := `consumes:
  "Bad;Svc":
    rest:
      "/users/{id}":
        patch:
          responses:
            600: Missing
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "service.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "consumes;Bad;Svc", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "Bad;Svc", "error": "must be snake_case"}, violations[0].Details)
}
