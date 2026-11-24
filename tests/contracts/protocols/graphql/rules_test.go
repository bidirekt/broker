package graphql_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
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
