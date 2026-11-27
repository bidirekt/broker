package graphql_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

func vocabularyViolations(t *testing.T, source string) []violation.Violation {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	grammar := protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}).Grammar()

	return descriptor.Validate(grammar, document, "web.yaml")
}

func TestSchemaVocabulary_AScalarWithANameAndPropertyArguments_AreValid(t *testing.T) {
	violations := vocabularyViolations(t, `schemas:
  DateTime: {type: scalar, name: DateTime}
  Pet:
    type: object
    properties:
      bornAt: {type: scalar, name: DateTime}
      reviews:
        type: array
        items: {type: scalar, name: Review}
        arguments: ReviewsArgs
      owner: {ref: Owner, arguments: OwnerArgs}
`)

	assert.Empty(t, violations)
}

func TestSchemaVocabulary_AScalarWithoutName_IsRefusedAtTheSchema(t *testing.T) {
	violations := vocabularyViolations(t, `schemas:
  DateTime: {type: scalar}
  Pet:
    type: object
    properties:
      bornAt: {type: scalar, name: ""}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.scalar_without_name", Path: "schemas;DateTime", Source: "web.yaml"},
		{ErrorCode: "schema.scalar_without_name", Path: "schemas;Pet;properties;bornAt", Source: "web.yaml"},
	}, violations)
}

func TestSchemaVocabulary_ScalarJoinsTheAllowedTypes(t *testing.T) {
	violations := vocabularyViolations(t, "schemas:\n  Pet: {type: strng}\n")

	require.Len(t, violations, 1)
	assert.Equal(t, "object, array, map, string, integer, float, number, boolean, enum, union, scalar", violations[0].Details["allowed"])
}

func TestSchemaVocabulary_ANameOnAnotherType_IsNotAllowed(t *testing.T) {
	violations := vocabularyViolations(t, "schemas:\n  Pet: {type: string, name: DateTime}\n")

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.key_not_allowed", Path: "schemas;Pet", Source: "web.yaml", Details: map[string]any{"key": "name", "type": "string"}},
	}, violations)
}

func TestSchemaVocabulary_ArgumentsOutsideAProperty_IsUnknown(t *testing.T) {
	violations := vocabularyViolations(t, `schemas:
  Pets:
    type: array
    items: {ref: Pet, arguments: PetArgs}
    arguments: PetsArgs
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "key.unknown", Path: "schemas;Pets;arguments", Source: "web.yaml", Details: map[string]any{"key": "arguments"}},
		{ErrorCode: "key.unknown", Path: "schemas;Pets;items;arguments", Source: "web.yaml", Details: map[string]any{"key": "arguments"}},
	}, violations)
}

func TestSchemaVocabulary_AUnionWithPropertiesAndMembers_IsValid(t *testing.T) {
	violations := vocabularyViolations(t, `schemas:
  Node:
    type: union
    properties:
      id: {type: string}
    members:
      User: {ref: UserName}
      Pet: {ref: PetSpecies}
  UserName: {type: object, properties: {name: {type: string}}}
  PetSpecies: {type: object, properties: {species: {type: string}}}
`)

	assert.Empty(t, violations)
}

func TestSchemaVocabulary_AUnionWithPropertiesButNoMembers_StillNeedsItsMembers(t *testing.T) {
	violations := vocabularyViolations(t, "schemas:\n  Node: {type: union, properties: {id: {type: string}}}\n")

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "members.required", Path: "schemas;Node", Source: "web.yaml", Details: map[string]any{"type": "union"}},
	}, violations)
}

func restAndGraphQLViolations(t *testing.T, contract string) []violation.Violation {
	t.Helper()

	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	violations, readErr := interpreter.ToViolations([]model.ContractFile{{Source: "api.yaml", Content: contract}})
	require.NoError(t, readErr)

	return violations
}

const allowedWithScalar = "object, array, map, string, integer, float, number, boolean, enum, union, scalar"

func TestSchemaVocabulary_PropertiesUnderAMistypedObject_AreStillValidated(t *testing.T) {
	violations := restAndGraphQLViolations(t, `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: objct
    properties:
      id: {type: strng}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.invalid_type", Path: "schemas;Pet;properties;id;type", Source: "api.yaml", Details: map[string]any{"value": "strng", "allowed": allowedWithScalar}},
		{ErrorCode: "schema.invalid_type", Path: "schemas;Pet;type", Source: "api.yaml", Details: map[string]any{"value": "objct", "allowed": allowedWithScalar}},
	}, violations)
}

func TestSchemaVocabulary_PropertiesBesideARef_AreStillValidated(t *testing.T) {
	violations := restAndGraphQLViolations(t, `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    ref: Other
    properties:
      id: {type: strng}
  Other: {type: object, properties: {id: {type: string}}}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.invalid_type", Path: "schemas;Pet;properties;id;type", Source: "api.yaml", Details: map[string]any{"value": "strng", "allowed": allowedWithScalar}},
		{ErrorCode: "schema.key_not_allowed", Path: "schemas;Pet", Source: "api.yaml", Details: map[string]any{"key": "properties", "type": "ref"}},
	}, violations)
}

func TestSchemaVocabulary_AUnionWithPropertiesRESTReaches_IsAKeyNotAllowed(t *testing.T) {
	violations := restAndGraphQLViolations(t, `provides:
  rest:
    /nodes:
      get:
        responses:
          200: Node
schemas:
  Node:
    type: union
    properties:
      id: {type: string}
    members:
      User: {ref: UserName}
  UserName: {type: object, properties: {name: {type: string}}}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.key_not_allowed", Path: "schemas;Node", Source: "api.yaml", Details: map[string]any{"key": "properties", "type": "union"}},
	}, violations)
}

func TestSchemaVocabulary_AUnionWithPropertiesAGraphQLConsumerReads_IsValid(t *testing.T) {
	violations := restAndGraphQLViolations(t, `consumes:
  petstore_api:
    graphql:
      query:
        node: {returns: MaybeNode}
schemas:
  MaybeNode: {ref: Node, nullable: true}
  Node:
    type: union
    properties:
      id: {type: string}
    members:
      User: {ref: UserName}
  UserName: {type: object, properties: {name: {type: string}}}
`)

	assert.Empty(t, violations)
}
