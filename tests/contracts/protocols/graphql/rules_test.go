package graphql_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const rulesSchemas = `
Pet:
  type: object
  properties:
    bornAt: {type: scalar, name: DateTime}
    owner: {ref: Owner, nullable: true}
    reviews:
      type: array
      items: {ref: Review}
      arguments: ReviewsArgs
Owner:
  type: object
  properties:
    since: {type: scalar, name: Date}
Review:
  type: object
  properties:
    stars: {type: integer}
ReviewsArgs:
  type: object
  properties:
    first: {type: scalar, name: Count}
Unreached:
  type: object
  properties:
    at: {type: scalar, name: DateTime}
`

func rulesDeclarations(t *testing.T, resources ...protocol.ResourceDeclaration) protocol.Declarations {
	t.Helper()

	var schemas map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(rulesSchemas), &schemas))

	declarations := protocol.Declarations{Resources: resources, Catalog: map[string]contract.SchemasMap{model.YAMLNamespace: {}}}
	for _, name := range contract.Document(schemas).Keys() {
		schema := contract.SchemaFromDocument(contract.Document(schemas).Mapping(name))
		declarations.Schemas = append(declarations.Schemas, protocol.SchemaDeclaration{Source: "schemas.yaml", Namespace: model.YAMLNamespace, Name: name, Schema: schema})
		declarations.Catalog[model.YAMLNamespace][name] = schema
	}

	return declarations
}

func rulesResource(resourceProtocol model.Protocol, schemaName string) protocol.ResourceDeclaration {
	return protocol.ResourceDeclaration{
		Source:     "web.yaml",
		Namespace:  model.YAMLNamespace,
		SchemaName: schemaName,
		Resource:   model.UploadedResource{Resource: model.Resource{Direction: model.Consumes, Protocol: resourceProtocol, Body: model.Output}},
	}
}

func rulesViolations(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation
	for _, rule := range graphqlcontracts.New().Rules() {
		violations = append(violations, rule(declarations)...)
	}

	return violation.SortedByLocation(violations)
}

func TestRules_ScalarsAndArgumentsAnotherProtocolReaches_AreRefusedWhereWritten(t *testing.T) {
	declarations := rulesDeclarations(t,
		rulesResource("rest", "Pet"),
		rulesResource("rest", "Owner"),
	)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.scalar_not_allowed", Path: "schemas;Owner;properties;since;type", Source: "schemas.yaml", Details: map[string]string{"protocol": "rest"}},
		{ErrorCode: "schema.scalar_not_allowed", Path: "schemas;Pet;properties;bornAt;type", Source: "schemas.yaml", Details: map[string]string{"protocol": "rest"}},
		{ErrorCode: "schema.arguments_not_allowed", Path: "schemas;Pet;properties;reviews;arguments", Source: "schemas.yaml", Details: map[string]string{"protocol": "rest"}},
	}, rulesViolations(declarations))
}

func TestRules_SchemasOnlyGraphQLReaches_AreAllowed(t *testing.T) {
	declarations := rulesDeclarations(t, rulesResource(graphqlmodel.Protocol, "Pet"))

	assert.Empty(t, rulesViolations(declarations))
}

func TestRules_AResourceWithoutSchema_ReachesNothing(t *testing.T) {
	undescribed := rulesResource("rest", "")
	undescribed.WithoutSchema = true

	assert.Empty(t, rulesViolations(rulesDeclarations(t, undescribed)))
}

func TestRules_AScalarAtTheTopOfASchema_IsRefusedAtItsType(t *testing.T) {
	declarations := protocol.Declarations{
		Resources: []protocol.ResourceDeclaration{rulesResource("rest", "BornAt")},
		Schemas:   []protocol.SchemaDeclaration{{Source: "api.yaml", Namespace: model.YAMLNamespace, Name: "BornAt", Schema: contract.Schema{Type: model.ScalarType, Scalar: "DateTime"}}},
		Catalog:   map[string]contract.SchemasMap{model.YAMLNamespace: {"BornAt": {Type: model.ScalarType, Scalar: "DateTime"}}},
	}

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.scalar_not_allowed", Path: "schemas;BornAt;type", Source: "api.yaml", Details: map[string]string{"protocol": "rest"}},
	}, rulesViolations(declarations))
}

func argumentsViolations(t *testing.T, consumer string) []violation.Violation {
	t.Helper()

	interpreter := contractfiles.NewInterpreter(
		protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
		[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
	)

	violations, readErr := interpreter.ToViolations([]model.ContractFile{{Source: "petstore_web.yaml", Content: consumer}})
	require.NoError(t, readErr)

	return violations
}

func TestRules_ArgumentsOfAConsumedFieldNamingAnythingButAnObject_AreRefusedWithTheTypeTheyName(t *testing.T) {
	cases := map[string]struct {
		arguments string
		schemas   string
		details   map[string]string
	}{
		"a string": {
			arguments: "PetId",
			schemas:   "  PetId: {type: string}\n",
			details:   map[string]string{"schema": "PetId", "reason": "not_object", "type": "string"},
		},
		"a ref to an array": {
			arguments: "PetArgs",
			schemas:   "  PetArgs: {ref: PetIds}\n  PetIds: {type: array, items: {type: string}}\n",
			details:   map[string]string{"schema": "PetArgs", "reason": "not_object", "type": "array"},
		},
		"a nullable object": {
			arguments: "MaybeArgs",
			schemas:   "  MaybeArgs: {type: object, nullable: true, properties: {id: {type: string}}}\n",
			details:   map[string]string{"schema": "MaybeArgs", "reason": "nullable"},
		},
		"a nullable ref to an object": {
			arguments: "MaybeArgs",
			schemas:   "  MaybeArgs: {ref: PetArgs, nullable: true}\n  PetArgs: {type: object, properties: {id: {type: string}}}\n",
			details:   map[string]string{"schema": "MaybeArgs", "reason": "nullable"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			consumer := "consumes:\n  petstore_api:\n    graphql:\n      query:\n        pet: {arguments: " + tc.arguments + ", returns: Pet}\n" +
				"schemas:\n  Pet: {type: object, properties: {name: {type: string}}}\n" + tc.schemas

			assert.Equal(t, []violation.Violation{{
				ErrorCode: "schema.arguments_not_object",
				Path:      "consumes;petstore_api;graphql;query;pet;arguments",
				Source:    "petstore_web.yaml",
				Details:   tc.details,
			}}, argumentsViolations(t, consumer))
		})
	}
}

func TestRules_ArgumentsOfAPropertyNamingAnythingButAnObject_AreRefusedWhereWritten(t *testing.T) {
	cases := map[string]struct {
		schemas string
		details map[string]string
	}{
		"an integer": {
			schemas: "  ReviewsArgs: {type: integer}\n",
			details: map[string]string{"schema": "ReviewsArgs", "reason": "not_object", "type": "integer"},
		},
		"a ref to a nullable object": {
			schemas: "  ReviewsArgs: {ref: MaybeArgs}\n  MaybeArgs: {type: object, nullable: true, properties: {first: {type: integer}}}\n",
			details: map[string]string{"schema": "ReviewsArgs", "reason": "nullable"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			consumer := "consumes:\n  petstore_api:\n    graphql:\n      query:\n        pet: {returns: Pet}\n" +
				"schemas:\n  Pet:\n    type: object\n    properties:\n      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}\n" + tc.schemas

			assert.Equal(t, []violation.Violation{{
				ErrorCode: "schema.arguments_not_object",
				Path:      "schemas;Pet;properties;reviews;arguments",
				Source:    "petstore_web.yaml",
				Details:   tc.details,
			}}, argumentsViolations(t, consumer))
		})
	}
}

func TestRules_ArgumentsNamingANonNullableObject_AreAllowed(t *testing.T) {
	assert.Empty(t, argumentsViolations(t, `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: Pet}
        pets: {returns: PetList}
schemas:
  PetArgs: {ref: IdArgs}
  IdArgs: {type: object, properties: {id: {type: string}}}
  PetList: {type: array, items: {ref: Pet}}
  Pet:
    type: object
    properties:
      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}
  ReviewsArgs: {type: object, properties: {first: {type: integer}}}
`))
}

func TestRules_ArgumentsAnotherProtocolReaches_AreRefusedAsNotAllowedOnly(t *testing.T) {
	assert.Equal(t, []violation.Violation{{
		ErrorCode: "schema.arguments_not_allowed",
		Path:      "schemas;Pet;properties;reviews;arguments",
		Source:    "petstore_web.yaml",
		Details:   map[string]string{"protocol": "rest"},
	}}, argumentsViolations(t, `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      reviews: {type: array, items: {type: string}, arguments: ReviewsArgs}
  ReviewsArgs: {type: integer}
`))
}
