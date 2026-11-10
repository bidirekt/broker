package descriptor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func TestServiceName_ExigeSnakeCase(t *testing.T) {
	subject := descriptor.Map(descriptor.ServiceName, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"Billing-API": nil, "billing_api": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "service.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "Billing-API", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "Billing-API", "error": "must be snake_case"}, violations[0].Details)
}

func TestSchemaType_AcceptsEveryAllowedType(t *testing.T) {
	for _, allowed := range []string{"object", "array", "map", "string", "integer", "float", "number", "boolean"} {
		assert.Empty(t, descriptor.Validate(descriptor.SchemaType, allowed, "api.yaml"), allowed)
	}
}

func TestSchemaType_RejectsATypeOutsideTheSet(t *testing.T) {
	violations := descriptor.Validate(descriptor.SchemaType, "decimal", "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, map[string]string{
		"value":   "decimal",
		"allowed": "object, array, map, string, integer, float, number, boolean, enum, union",
	}, violations[0].Details)
}

func TestFlag_ExigeBooleano(t *testing.T) {
	violations := descriptor.Validate(descriptor.Flag, "talvez", "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, map[string]string{"expected": "boolean", "got": "string"}, violations[0].Details)
}

func TestPropertyName_RejectsAPathCharacterInAnyPosition(t *testing.T) {
	subject := descriptor.Map(descriptor.PropertyName, descriptor.Object(descriptor.Fields{}))

	for _, name := range []string{".name", "user.name", "name.", "[tags", "tags[]", "a]b", "a{b", "pets}", ";y", "x;y", "card(x)", "a)b"} {
		t.Run(name, func(t *testing.T) {
			violations := descriptor.Validate(subject, map[string]any{name: nil}, "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
			assert.Equal(t, name, violations[0].Path)
			assert.Equal(t, map[string]string{"key": name, "error": `must not contain ".", "[", "]", "{", "}", ";", "(" or ")"`}, violations[0].Details)
		})
	}
}

func TestPropertyName_RejectsAnEmptyName(t *testing.T) {
	subject := descriptor.Map(descriptor.PropertyName, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "", "error": "must not be empty"}, violations[0].Details)
}

func TestPropertyName_RejectsControlCharacters(t *testing.T) {
	subject := descriptor.Map(descriptor.PropertyName, descriptor.Object(descriptor.Fields{}))

	for _, name := range []string{"a\x00b", "a\tb"} {
		t.Run(name, func(t *testing.T) {
			violations := descriptor.Validate(subject, map[string]any{name: nil}, "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "property.name_syntax", violations[0].ErrorCode)
			assert.Equal(t, name, violations[0].Path)
			assert.Equal(t, map[string]string{"key": name, "error": "must not contain control characters"}, violations[0].Details)
		})
	}
}

func TestPropertyName_ReportsAPathCharacterBeforeAControlCharacter(t *testing.T) {
	subject := descriptor.Map(descriptor.PropertyName, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"user.\tname": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, `must not contain ".", "[", "]", "{", "}", ";", "(" or ")"`, violations[0].Details["error"])
}

func TestPropertyName_AcceptsNamesFromRealBodies(t *testing.T) {
	subject := descriptor.Map(descriptor.PropertyName, descriptor.Object(descriptor.Fields{}))

	for _, name := range []string{"content-type", "@type", "$ref", "first name", "petId", "*"} {
		assert.Empty(t, descriptor.Validate(subject, map[string]any{name: nil}, "api.yaml"), name)
	}
}

func TestSchemaName_RejectsAPathCharacterInAnyPosition(t *testing.T) {
	subject := descriptor.Map(descriptor.SchemaName, descriptor.Object(descriptor.Fields{}))

	for _, name := range []string{".Pet", "Pet.v2", "Pet.", "[Pets", "Pets[]", "Pet]s", "Pet{s", "Pets}", ";Pet", "Pet;v2", "Pet(v2)", "Pet)"} {
		t.Run(name, func(t *testing.T) {
			violations := descriptor.Validate(subject, map[string]any{name: nil}, "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
			assert.Equal(t, name, violations[0].Path)
			assert.Equal(t, map[string]string{"key": name, "error": `must not contain ".", "[", "]", "{", "}", ";", "(" or ")"`}, violations[0].Details)
		})
	}
}

func TestSchemaName_RejectsAnEmptyName(t *testing.T) {
	subject := descriptor.Map(descriptor.SchemaName, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
	assert.Equal(t, "", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "", "error": "must not be empty"}, violations[0].Details)
}

func TestSchemaName_RejectsControlCharacters(t *testing.T) {
	subject := descriptor.Map(descriptor.SchemaName, descriptor.Object(descriptor.Fields{}))

	for _, name := range []string{"Pet\x00", "Pet\tv2"} {
		t.Run(name, func(t *testing.T) {
			violations := descriptor.Validate(subject, map[string]any{name: nil}, "api.yaml")

			require.Len(t, violations, 1)
			assert.Equal(t, "schema.name_syntax", violations[0].ErrorCode)
			assert.Equal(t, name, violations[0].Path)
			assert.Equal(t, map[string]string{"key": name, "error": "must not contain control characters"}, violations[0].Details)
		})
	}
}

func TestSchemaName_AcceptsUnderscoreAndHyphen(t *testing.T) {
	subject := descriptor.Map(descriptor.SchemaName, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"Pet_V2": nil, "pet-record": nil}, "api.yaml")

	assert.Empty(t, violations)
}
