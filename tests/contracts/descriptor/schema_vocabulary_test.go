package descriptor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func scalarVocabulary() descriptor.Vocabulary {
	vocabulary := descriptor.CoreVocabulary()
	vocabulary.Types = append(vocabulary.Types, descriptor.TypeVocabulary{
		Name: "scalar",
		Keys: descriptor.Keys{"name": func(descriptor.Vocabulary) descriptor.Node { return descriptor.String() }},
		Rules: []descriptor.Rule{func(present descriptor.PresentFields) *violation.Violation {
			if present["type"] != "scalar" || present["name"] != nil {
				return nil
			}

			return &violation.Violation{ErrorCode: "schema.scalar_without_name"}
		}},
	})
	vocabulary.PropertyKeys = descriptor.Keys{"arguments": func(descriptor.Vocabulary) descriptor.Node { return descriptor.SchemaRef }}

	return vocabulary
}

var nestedScalarDocument = map[string]any{"schemas": map[string]any{
	"Pet": map[string]any{"type": "object", "properties": map[string]any{
		"owner": map[string]any{"type": "object", "properties": map[string]any{
			"bornAt":  map[string]any{"type": "scalar", "name": "DateTime"},
			"reviews": map[string]any{"type": "string", "arguments": "ReviewsArguments"},
		}},
	}},
}}

func TestContract_TwoVocabulariesInOneProcess_EachValidatesWithItsOwn(t *testing.T) {
	scalarGrammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, scalarVocabulary())
	coreGrammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, descriptor.CoreVocabulary())

	assert.Empty(t, descriptor.Validate(scalarGrammar, nestedScalarDocument, "api.yaml"))
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;Pet;properties;owner;properties;bornAt;name",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "name"},
		},
		{
			ErrorCode: "schema.invalid_type",
			Path:      "schemas;Pet;properties;owner;properties;bornAt;type",
			Source:    "api.yaml",
			Details: map[string]any{
				"value":   "scalar",
				"allowed": "object, array, map, string, integer, float, number, boolean, enum, union",
			},
		},
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;Pet;properties;owner;properties;reviews;arguments",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "arguments"},
		},
	}, descriptor.Validate(coreGrammar, nestedScalarDocument, "api.yaml"))
}

func TestContract_TypeAddedByAVocabulary_BringsItsKeysAndRules(t *testing.T) {
	grammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, scalarVocabulary())

	violations := descriptor.Validate(grammar, map[string]any{"schemas": map[string]any{
		"DateTime": map[string]any{"type": "scalar"},
		"Name":     map[string]any{"type": "string", "name": "Name"},
	}}, "api.yaml")

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.scalar_without_name",
			Path:      "schemas;DateTime",
			Source:    "api.yaml",
		},
		{
			ErrorCode: "schema.key_not_allowed",
			Path:      "schemas;Name",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "name", "type": "string"},
		},
	}, violations)
}

func TestContract_PropertyKeyAddedByAVocabulary_NotTakenOutsideAProperty(t *testing.T) {
	grammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, scalarVocabulary())

	violations := descriptor.Validate(grammar, map[string]any{"schemas": map[string]any{
		"Pets": map[string]any{"type": "array", "items": map[string]any{"type": "string", "arguments": "PetsArguments"}},
	}}, "api.yaml")

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;Pets;items;arguments",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "arguments"},
		},
	}, violations)
}

func propertiesTakingVocabulary() descriptor.Vocabulary {
	vocabulary := descriptor.CoreVocabulary()
	for index, schemaType := range vocabulary.Types {
		if schemaType.Name == "union" {
			vocabulary.Types[index].Takes = []string{"properties"}
		}
	}

	return vocabulary
}

func TestContract_TypeTakingAKeyOfAnotherType_ValidatesItWithThatGrammar(t *testing.T) {
	grammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, propertiesTakingVocabulary())

	violations := descriptor.Validate(grammar, map[string]any{"schemas": map[string]any{
		"Node": map[string]any{
			"type":       "union",
			"properties": map[string]any{"id": map[string]any{"type": "strng"}},
			"members":    map[string]any{"User": map[string]any{"ref": "User"}},
		},
	}}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "schema.invalid_type", violations[0].ErrorCode)
	assert.Equal(t, "schemas;Node;properties;id;type", violations[0].Path)
}

func TestContract_KeyTakenByAnotherType_IsStillValidatedUnderATypeThatDoesNotTakeIt(t *testing.T) {
	document := map[string]any{"schemas": map[string]any{
		"Pet": map[string]any{"type": "string", "properties": map[string]any{"id": map[string]any{"type": "strng"}}},
		"Pal": map[string]any{"ref": "Pet", "properties": map[string]any{"id": map[string]any{"type": "strng"}}},
	}}
	coreGrammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, descriptor.CoreVocabulary())
	takingGrammar := descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, propertiesTakingVocabulary())

	expected := []violation.Violation{
		{
			ErrorCode: "schema.invalid_type",
			Path:      "schemas;Pal;properties;id;type",
			Source:    "api.yaml",
			Details:   map[string]any{"value": "strng", "allowed": "object, array, map, string, integer, float, number, boolean, enum, union"},
		},
		{
			ErrorCode: "schema.key_not_allowed",
			Path:      "schemas;Pal",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "properties", "type": "ref"},
		},
		{
			ErrorCode: "schema.invalid_type",
			Path:      "schemas;Pet;properties;id;type",
			Source:    "api.yaml",
			Details:   map[string]any{"value": "strng", "allowed": "object, array, map, string, integer, float, number, boolean, enum, union"},
		},
		{
			ErrorCode: "schema.key_not_allowed",
			Path:      "schemas;Pet",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "properties", "type": "string"},
		},
	}
	assert.Equal(t, expected, descriptor.Validate(coreGrammar, document, "api.yaml"))
	assert.Equal(t, expected, descriptor.Validate(takingGrammar, document, "api.yaml"))
}

func TestContract_TypeTakingAKeyOfSeveralGrammars_Panics(t *testing.T) {
	vocabulary := descriptor.CoreVocabulary()
	vocabulary.Types = append(vocabulary.Types, descriptor.TypeVocabulary{Name: "scalar", Takes: []string{"members"}})

	assert.PanicsWithError(t, `schema type "scalar" takes key "members", which has no single grammar`, func() {
		descriptor.Contract(descriptor.Fields{}, descriptor.Fields{}, vocabulary)
	})
}
