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
		"      type: number\n"

	violations := descriptor.Validate(descriptor.Schema(), schemaDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "properties;tags;items;type", violations[0].Path)
	assert.Equal(t, "number", violations[0].Details["value"])
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

func TestSchema_VazioEhTipoInvalido(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), map[string]any{}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, map[string]string{
		"value":   "",
		"allowed": "object, array, string, integer, float, boolean",
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

func TestSchema_TipoForaDoConjuntoContaComoForma(t *testing.T) {
	violations := descriptor.Validate(descriptor.Schema(), map[string]any{"type": "number"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "type", violations[0].Path)
}
