package descriptor_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func schemaDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestSchema_RecursaoDesceEmProfundidade(t *testing.T) {
	source := "type: object\n" +
		"properties:\n" +
		"  tags:\n" +
		"    type: array\n" +
		"    items:\n" +
		"      type: decimal\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "properties;tags;items;type", violations[0].Path)
	assert.Equal(t, "decimal", violations[0].Details["value"])
}

func TestSchema_ArrayDeArrayDeObjetoValido(t *testing.T) {
	source := "type: array\n" +
		"items:\n" +
		"  type: array\n" +
		"  items:\n" +
		"    type: object\n" +
		"    properties:\n" +
		"      name:\n" +
		"        type: string\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_ArraySemItemsNoNivelFundo(t *testing.T) {
	source := "type: object\n" +
		"properties:\n" +
		"  tags:\n" +
		"    type: array\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.array_without_items", violations[0].ErrorCode)
	assert.Equal(t, "properties;tags", violations[0].Path)
	assert.Nil(t, violations[0].Details)
}

func TestSchema_MapWithoutValuesAtTheDeepestLevel(t *testing.T) {
	source := "type: object\n" +
		"properties:\n" +
		"  pets:\n" +
		"    type: map\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.map_without_values", violations[0].ErrorCode)
	assert.Equal(t, "properties;pets", violations[0].Path)
	assert.Nil(t, violations[0].Details)
}

func TestSchema_EmptyIsAnInvalidType(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), map[string]any{}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, map[string]string{
		"value":   "",
		"allowed": "object, array, map, string, integer, float, number, boolean",
	}, violations[0].Details)
}

func TestSchema_NuloEhTipoInvalido(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), nil, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Details["value"])
}

func TestSchema_SoRefEhValido(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), map[string]any{"ref": "Pet"}, "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_RefWithOptionalNullableAndDescriptionIsValid(t *testing.T) {
	source := "ref: Person\n" +
		"optional: true\n" +
		"nullable: true\n" +
		"description: who looks after the pet\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_ShapeKeyBesideRefIsNotAllowed(t *testing.T) {
	const ownerRef = "type: object\n" +
		"properties:\n" +
		"  owner:\n" +
		"    ref: Person\n"

	for key, besideRef := range map[string]string{
		"type":       "    type: string\n",
		"properties": "    properties:\n      name:\n        type: string\n",
		"items":      "    items:\n      type: string\n",
		"values":     "    values:\n      type: string\n",
	} {
		t.Run(key, func(t *testing.T) {
			violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, ownerRef+besideRef), "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "schema.key_not_allowed", violations[0].ErrorCode)
			assert.Equal(t, "properties;owner", violations[0].Path)
			assert.Equal(t, map[string]string{"key": key, "type": "ref"}, violations[0].Details)
		})
	}
}

func TestSchema_EachShapeKeyBesideRefIsReportedOnItsOwn(t *testing.T) {
	source := "ref: Person\n" +
		"type: object\n" +
		"properties:\n" +
		"  name:\n" +
		"    type: string\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "schema.key_not_allowed", violations[0].ErrorCode)
	assert.Equal(t, map[string]string{"key": "type", "type": "ref"}, violations[0].Details)
	assert.Equal(t, "schema.key_not_allowed", violations[1].ErrorCode)
	assert.Equal(t, map[string]string{"key": "properties", "type": "ref"}, violations[1].Details)
}

func TestSchema_TipoForaDoConjuntoContaComoForma(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), map[string]any{"type": "decimal"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "type", violations[0].Path)
}

func TestSchema_NullableIsAcceptedOnEveryNode(t *testing.T) {
	source := "type: object\n" +
		"nullable: true\n" +
		"properties:\n" +
		"  avatar:\n" +
		"    type: string\n" +
		"    nullable: true\n" +
		"  tags:\n" +
		"    type: array\n" +
		"    items:\n" +
		"      type: string\n" +
		"      nullable: true\n" +
		"  manager:\n" +
		"    ref: User\n" +
		"    optional: true\n" +
		"    nullable: true\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_MapWithValuesIsValid(t *testing.T) {
	source := "type: map\n" +
		"values:\n" +
		"  ref: Pet\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_PropertiesItemsOrValuesWithoutTypeIsAnInvalidType(t *testing.T) {
	const attributesProperty = "type: object\n" +
		"properties:\n" +
		"  attributes:\n"

	for key, withoutType := range map[string]string{
		"properties": "    properties:\n      color:\n        type: string\n",
		"items":      "    items:\n      type: string\n",
		"values":     "    values:\n      type: string\n",
	} {
		t.Run(key, func(t *testing.T) {
			violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, attributesProperty+withoutType), "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
			assert.Equal(t, "properties;attributes", violations[0].Path)
			assert.Equal(t, map[string]string{
				"value":   "",
				"allowed": "object, array, map, string, integer, float, number, boolean",
			}, violations[0].Details)
		})
	}
}

func TestSchema_KeyOfAnotherTypeIsNotAllowed(t *testing.T) {
	for name, testCase := range map[string]struct {
		node         string
		key          string
		declaredType string
	}{
		"items on a string": {
			node:         "    type: string\n    items:\n      type: string\n",
			key:          "items",
			declaredType: "string",
		},
		"values on an object": {
			node:         "    type: object\n    properties:\n      bob:\n        type: string\n    values:\n      type: string\n",
			key:          "values",
			declaredType: "object",
		},
		"properties on a map": {
			node:         "    type: map\n    values:\n      type: string\n    properties:\n      bob:\n        type: string\n",
			key:          "properties",
			declaredType: "map",
		},
		"properties on an array": {
			node:         "    type: array\n    items:\n      type: string\n    properties:\n      bob:\n        type: string\n",
			key:          "properties",
			declaredType: "array",
		},
		"items on a map": {
			node:         "    type: map\n    values:\n      type: string\n    items:\n      type: string\n",
			key:          "items",
			declaredType: "map",
		},
		"values on an array": {
			node:         "    type: array\n    items:\n      type: string\n    values:\n      type: string\n",
			key:          "values",
			declaredType: "array",
		},
	} {
		t.Run(name, func(t *testing.T) {
			source := "type: object\n" +
				"properties:\n" +
				"  pets:\n" +
				testCase.node

			violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "schema.key_not_allowed", violations[0].ErrorCode)
			assert.Equal(t, "properties;pets", violations[0].Path)
			assert.Equal(t, map[string]string{"key": testCase.key, "type": testCase.declaredType}, violations[0].Details)
		})
	}
}

func TestSchema_EachKeyOfAnotherTypeIsReportedOnItsOwn(t *testing.T) {
	source := "type: integer\n" +
		"properties:\n" +
		"  bob:\n" +
		"    type: string\n" +
		"items:\n" +
		"  type: string\n" +
		"values:\n" +
		"  type: string\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	require.Len(t, violations, 3)
	for index, key := range []string{"properties", "items", "values"} {
		assert.Equal(t, "schema.key_not_allowed", violations[index].ErrorCode)
		assert.Equal(t, "", violations[index].Path)
		assert.Equal(t, map[string]string{"key": key, "type": "integer"}, violations[index].Details)
	}
}

func TestSchema_ObjectWithoutPropertiesOrValuesIsValid(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), map[string]any{"type": "object"}, "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_PropertyNamedStarIsAnOrdinaryName(t *testing.T) {
	source := "type: object\n" +
		"properties:\n" +
		"  \"*\":\n" +
		"    type: string\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestSchema_NullableIsAcceptedOnTheValuesNode(t *testing.T) {
	source := "type: map\n" +
		"values:\n" +
		"  type: string\n" +
		"  nullable: true\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}
