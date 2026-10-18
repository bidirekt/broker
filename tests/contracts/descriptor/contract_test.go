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

func TestContract_PatchWithRequestAndResponses_IsAcceptedInProvidesAndConsumes(t *testing.T) {
	source := `provides:
  rest:
    /pets/*:
      patch:
        request: PetPatch
        responses:
          200: Pet
consumes:
  pets_api:
    rest:
      /pets/*:
        patch:
          request: PetPatch
          responses:
            200: Pet
schemas:
  PetPatch:
    type: object
    properties:
      name:
        type: string
  Pet:
    type: object
    properties:
      id:
        type: string
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestContract_FiveErrorsInOnePass(t *testing.T) {
	source := `provides:
  rest:
    /pets:
      options:
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
    type: decimal
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
	assert.Equal(t, "provides;rest;/pets;options", violations[1].Path)
	assert.Equal(t, map[string]string{"key": "options"}, violations[1].Details)

	assert.Equal(t, "endpoint.syntax", violations[2].ErrorCode)
	assert.Equal(t, "provides;rest;/users/{id}", violations[2].Path)
	assert.Equal(t, "/users/{id}", violations[2].Details["key"])

	assert.Equal(t, "schema.invalid_type", violations[3].ErrorCode)
	assert.Equal(t, "schemas;Pet;type", violations[3].Path)
	assert.Equal(t, "decimal", violations[3].Details["value"])

	assert.Equal(t, "schema.array_without_items", violations[4].ErrorCode)
	assert.Equal(t, "schemas;Tags", violations[4].Path)
}

func TestContract_MessageViraChaveDesconhecida(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "provides:\n  message:\n    saudacao: ola\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "key.unknown", violations[0].ErrorCode)
	assert.Equal(t, "provides;message", violations[0].Path)
}

func TestContract_EmptySchemaIsAnInvalidType(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "schemas:\n  Pet: {}\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet", violations[0].Path)
	assert.Equal(t, "", violations[0].Details["value"])
	assert.Equal(t, "object, array, map, string, integer, float, number, boolean", violations[0].Details["allowed"])
}

func TestContract_SchemaNuloEhTipoInvalido(t *testing.T) {
	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, "schemas:\n  Pet:\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet", violations[0].Path)
}

func TestContract_NonBooleanOptionalIsAnInvalidKind(t *testing.T) {
	source := "schemas:\n  Pet:\n    type: object\n    properties:\n      id:\n        type: string\n        optional: maybe\n"

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet;properties;id;optional", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "boolean", "got": "string"}, violations[0].Details)
}

func TestContract_OptionalOnANamedSchemaIsNotAllowed(t *testing.T) {
	for _, optional := range []string{"true", "false"} {
		t.Run(optional, func(t *testing.T) {
			source := "schemas:\n  Pet:\n    type: object\n    optional: " + optional + "\n    properties:\n      id:\n        type: string\n"

			violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "schema.optional_not_allowed", violations[0].ErrorCode)
			assert.Equal(t, "schemas;Pet", violations[0].Path)
			assert.Equal(t, "api.yaml", violations[0].Source)
			assert.Equal(t, map[string]string{"position": "schema"}, violations[0].Details)
		})
	}
}

func TestContract_NullOptionalOrNullableOnANamedSchemaIsValid(t *testing.T) {
	for name, flag := range map[string]string{
		"null optional": "optional: ~",
		"nullable":      "nullable: true",
	} {
		t.Run(name, func(t *testing.T) {
			source := "schemas:\n  Pet:\n    type: object\n    " + flag + "\n    properties:\n      id:\n        type: string\n"

			violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

			assert.Empty(t, violations)
		})
	}
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

func TestContract_PropertyNameWithAPathCharacterIsRejected(t *testing.T) {
	for name, path := range map[string]string{
		"user.name": "schemas;Pet;properties;user.name",
		"tags[]":    "schemas;Pet;properties;tags[]",
		"a{b":       "schemas;Pet;properties;a{b",
		"x;y":       "schemas;Pet;properties;x;y",
	} {
		t.Run(name, func(t *testing.T) {
			source := "schemas:\n  Pet:\n    type: object\n    properties:\n      \"" + name + "\":\n        type: string\n"

			violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
			assert.Equal(t, path, violations[0].Path)
			assert.Equal(t, "api.yaml", violations[0].Source)
			assert.Equal(t, map[string]string{"key": name, "error": `must not contain ".", "[", "]", "{", "}" or ";"`}, violations[0].Details)
		})
	}
}

func TestContract_EmptyPropertyNameIsRejected(t *testing.T) {
	source := "schemas:\n  Pet:\n    type: object\n    properties:\n      \"\":\n        type: string\n"

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet;properties;", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "", "error": "must not be empty"}, violations[0].Details)
}

func TestContract_PropertyNameWithAControlCharacterIsRejected(t *testing.T) {
	source := `schemas:
  Pet:
    type: object
    properties:
      "a\x00b":
        type: string
      "a\tb":
        type: string
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet;properties;a\x00b", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "a\x00b", "error": "must not contain control characters"}, violations[0].Details)
	assert.Equal(t, "property.name_syntax", violations[1].ErrorCode)
	assert.Equal(t, "schemas;Pet;properties;a\tb", violations[1].Path)
	assert.Equal(t, map[string]string{"key": "a\tb", "error": "must not contain control characters"}, violations[1].Details)
}

func TestContract_RejectedPropertyIsReportedOnlyByItsName(t *testing.T) {
	source := "schemas:\n  Pet:\n    type: object\n    properties:\n      user.name:\n        type: decimal\n"

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet;properties;user.name", violations[0].Path)
}

func TestContract_SchemaNameWithAPathCharacterIsRejected(t *testing.T) {
	source := "schemas:\n  Pet.v2:\n    type: object\n    properties:\n      id:\n        type: string\n"

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet.v2", violations[0].Path)
	assert.Equal(t, "api.yaml", violations[0].Source)
	assert.Equal(t, map[string]string{"key": "Pet.v2", "error": `must not contain ".", "[", "]", "{", "}" or ";"`}, violations[0].Details)
}

func TestContract_EmptySchemaNameIsRejected(t *testing.T) {
	source := "schemas:\n  \"\":\n    type: object\n    properties:\n      id:\n        type: string\n"

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "", "error": "must not be empty"}, violations[0].Details)
}

func TestContract_SchemaNameWithAControlCharacterIsRejected(t *testing.T) {
	source := `schemas:
  "Pet\x00":
    type: object
    properties:
      id:
        type: string
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet\x00", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "Pet\x00", "error": "must not contain control characters"}, violations[0].Details)
}

func TestContract_RejectedSchemaIsReportedOnlyByItsName(t *testing.T) {
	source := `provides:
  rest:
    /pets:
      post:
        request: Pet.v2
schemas:
  Pet.v2:
    type: decimal
`

	violations := descriptor.Validate(descriptor.Contract, contractDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Pet.v2", violations[0].Path)
}
