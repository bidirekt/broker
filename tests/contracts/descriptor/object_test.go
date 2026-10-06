package descriptor_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func objectDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestObject_RejeitaChaveDesconhecida(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{
		"type":     descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object"}},
		"optional": descriptor.Bool(),
	})

	violations := descriptor.Validate(subject, objectDocument(t, "type: object\npatch: nope\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "key.unknown", violations[0].ErrorCode)
	assert.Equal(t, "patch", violations[0].Path)
	assert.Equal(t, "api.yaml", violations[0].Source)
	assert.Equal(t, map[string]string{"key": "patch"}, violations[0].Details)
}

func TestObject_AcumulaChavesDesconhecidasEmOrdemEstavel(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{})

	violations := descriptor.Validate(subject, objectDocument(t, "segunda: dois\nprimeira: um\n"), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "primeira", violations[0].Details["key"])
	assert.Equal(t, "segunda", violations[1].Details["key"])
}

func TestObject_DespachaCampoParaSeuNo(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{
		"type":     descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object", "array", "string"}},
		"optional": descriptor.Bool(),
	})

	violations := descriptor.Validate(subject, objectDocument(t, "type: number\npatch: nope\n"), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "key.unknown", violations[0].ErrorCode)
	assert.Equal(t, "schema.invalid_type", violations[1].ErrorCode)
}

func TestObject_CaminhoDesceComPontoEVirgula(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{
		"schemas": descriptor.Object(descriptor.Fields{
			"Pet": descriptor.Object(descriptor.Fields{
				"type": descriptor.Enum{ErrorCode: "schema.invalid_type", Allowed: []string{"object"}},
			}),
		}),
	})

	violations := descriptor.Validate(subject, objectDocument(t, "schemas:\n  Pet:\n    type: number\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schemas;Pet;type", violations[0].Path)
}

func TestObject_NuloContaComoVazio(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{"rest": descriptor.String()})

	violations := descriptor.Validate(subject, nil, "api.yaml")

	assert.Empty(t, violations)
}

func TestObject_RejeitaEscalarComoKind(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{})

	violations := descriptor.Validate(subject, "nope", "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, map[string]string{"expected": "mapping", "got": "string"}, violations[0].Details)
}

func TestObject_CampoComValorNuloNaoContaComoEscrito(t *testing.T) {
	var seen descriptor.PresentFields

	subject := descriptor.Object(descriptor.Fields{"optional": descriptor.Bool()}).
		Rules(func(present descriptor.PresentFields) *violation.Violation {
			seen = present

			return nil
		})

	violations := descriptor.Validate(subject, objectDocument(t, "optional:\n"), "api.yaml")

	assert.Empty(t, violations)
	assert.Empty(t, seen)
}

func TestObject_RegrasRodamDepoisDosCamposERecebemOsEscritos(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{
		"type":  descriptor.String(),
		"items": descriptor.String(),
	}).Rules(func(present descriptor.PresentFields) *violation.Violation {
		if present["type"] != "array" || present.HasAny("items") {
			return nil
		}

		return &violation.Violation{ErrorCode: "schema.array_without_items"}
	})

	violations := descriptor.Validate(subject, objectDocument(t, "type: array\nextra: x\n"), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "key.unknown", violations[0].ErrorCode)
	assert.Equal(t, "schema.array_without_items", violations[1].ErrorCode)
	assert.Equal(t, "", violations[1].Path)
	assert.Equal(t, "api.yaml", violations[1].Source)
}

func TestObject_RegraEmObjetoNuloRodaComEscritosVazios(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{
		"Pet": descriptor.Object(descriptor.Fields{"type": descriptor.String()}).
			Rules(func(present descriptor.PresentFields) *violation.Violation {
				if present.HasAny("type") {
					return nil
				}

				return &violation.Violation{ErrorCode: "schema.invalid_type"}
			}),
	})

	violations := descriptor.Validate(subject, objectDocument(t, "Pet:\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "Pet", violations[0].Path)
}

func TestObject_RegraNaoTemSeuPonteiroMutado(t *testing.T) {
	shared := &violation.Violation{ErrorCode: "schema.invalid_type"}

	subject := descriptor.Object(descriptor.Fields{
		"Pet": descriptor.Object(descriptor.Fields{}).
			Rules(func(descriptor.PresentFields) *violation.Violation { return shared }),
	})

	violations := descriptor.Validate(subject, objectDocument(t, "Pet: {}\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "Pet", violations[0].Path)
	assert.Equal(t, "", shared.Path)
	assert.Equal(t, "", shared.Source)
}
