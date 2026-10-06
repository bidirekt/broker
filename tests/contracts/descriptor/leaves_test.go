package descriptor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func TestString_AceitaString(t *testing.T) {
	violations := descriptor.Validate(descriptor.String(), "Pet", "api.yaml")

	assert.Empty(t, violations)
}

func TestString_NuloContaComoAusente(t *testing.T) {
	violations := descriptor.Validate(descriptor.String(), nil, "api.yaml")

	assert.Empty(t, violations)
}

func TestString_RejeitaMapeamento(t *testing.T) {
	violations := descriptor.Validate(descriptor.String(), map[string]any{"x": "um"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, "api.yaml", violations[0].Source)
	assert.Equal(t, map[string]string{"expected": "string", "got": "mapping"}, violations[0].Details)
}

func TestBool_AceitaBooleano(t *testing.T) {
	violations := descriptor.Validate(descriptor.Bool(), true, "api.yaml")

	assert.Empty(t, violations)
}

func TestBool_RejeitaString(t *testing.T) {
	violations := descriptor.Validate(descriptor.Bool(), "nope", "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, map[string]string{"expected": "boolean", "got": "string"}, violations[0].Details)
}

func TestEnum_AceitaValorDoConjunto(t *testing.T) {
	subject := descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object", "array"}}

	violations := descriptor.Validate(subject, "array", "api.yaml")

	assert.Empty(t, violations)
}

func TestEnum_RejeitaValorForaDoConjunto(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{
		"type": descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object", "array"}},
	})

	violations := descriptor.Validate(subject, map[string]any{"type": "number"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "type", violations[0].Path)
	assert.Equal(t, map[string]string{"value": "number", "allowed": "object, array"}, violations[0].Details)
}

func TestEnum_JulgaEscalarPeloTexto(t *testing.T) {
	subject := descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object", "200"}}

	assert.Empty(t, descriptor.Validate(subject, uint64(200), "api.yaml"))

	violations := descriptor.Validate(subject, true, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "true", violations[0].Details["value"])
}

func TestEnum_RejeitaMapeamentoComoKind(t *testing.T) {
	subject := descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object"}}

	violations := descriptor.Validate(subject, map[string]any{"x": "um"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, map[string]string{"expected": "string", "got": "mapping"}, violations[0].Details)
}

func TestEnum_NuloContaComoAusente(t *testing.T) {
	subject := descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object"}}

	violations := descriptor.Validate(subject, nil, "api.yaml")

	assert.Empty(t, violations)
}
