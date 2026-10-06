package descriptor_test

import (
	"errors"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func mapDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func atLeastThreeLetters(key string) error {
	if len(key) < 3 {
		return errors.New("too short")
	}

	return nil
}

func TestMap_TodasAsChavesSeguemOMesmoNo(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{}, descriptor.Object(descriptor.Fields{
		"type": descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object", "array"}},
	}))

	violations := descriptor.Validate(subject, mapDocument(t, "Pet:\n  type: object\nOwner:\n  type: number\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "Owner;type", violations[0].Path)
}

func TestMap_ChaveSemRegraAceitaQualquerTexto(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{}, descriptor.String())

	violations := descriptor.Validate(subject, mapDocument(t, "\"/users/{id}\": a\nBilling-API: b\n600: c\n"), "api.yaml")

	assert.Empty(t, violations)
}

func TestMap_ChaveReprovadaSaiComCodigoChaveEErro(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{ErrorCode: "key.short", Check: atLeastThreeLetters}, descriptor.String())

	violations := descriptor.Validate(subject, mapDocument(t, "ab: x\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "key.short", violations[0].ErrorCode)
	assert.Equal(t, "ab", violations[0].Path)
	assert.Equal(t, "api.yaml", violations[0].Source)
	assert.Equal(t, map[string]string{"key": "ab", "error": "too short"}, violations[0].Details)
}

func TestMap_ChaveReprovadaNaoDesceNoValor(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{ErrorCode: "key.short", Check: atLeastThreeLetters}, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, mapDocument(t, "ab:\n  patch: nope\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "key.short", violations[0].ErrorCode)
}

func TestMap_ChavesSaemEmOrdemEstavel(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{}, descriptor.String())

	violations := descriptor.Validate(subject, mapDocument(t, "beta: {}\nalfa: {}\n"), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "alfa", violations[0].Path)
	assert.Equal(t, "beta", violations[1].Path)
}

func TestMap_NuloContaComoVazio(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{ErrorCode: "key.short", Check: atLeastThreeLetters}, descriptor.String())

	violations := descriptor.Validate(subject, nil, "api.yaml")

	assert.Empty(t, violations)
}

func TestMap_RejeitaSequenciaComoKind(t *testing.T) {
	subject := descriptor.Map(descriptor.Key{}, descriptor.String())

	violations := descriptor.Validate(subject, []any{"a", "b"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "mapping", "got": "sequence"}, violations[0].Details)
}
