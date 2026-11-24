package descriptor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
	scalarGrammar := descriptor.Contract(descriptor.Fields{}, scalarVocabulary())
	coreGrammar := descriptor.Contract(descriptor.Fields{}, descriptor.CoreVocabulary())

	assert.Empty(t, descriptor.Validate(scalarGrammar, nestedScalarDocument, "api.yaml"))
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;Pet;properties;owner;properties;bornAt;name",
			Source:    "api.yaml",
			Details:   map[string]string{"key": "name"},
		},
		{
			ErrorCode: "schema.invalid_type",
			Path:      "schemas;Pet;properties;owner;properties;bornAt;type",
			Source:    "api.yaml",
			Details: map[string]string{
				"value":   "scalar",
				"allowed": "object, array, map, string, integer, float, number, boolean, enum, union",
			},
		},
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;Pet;properties;owner;properties;reviews;arguments",
			Source:    "api.yaml",
			Details:   map[string]string{"key": "arguments"},
		},
	}, descriptor.Validate(coreGrammar, nestedScalarDocument, "api.yaml"))
}

func TestContract_TypeAddedByAVocabulary_BringsItsKeysAndRules(t *testing.T) {
	grammar := descriptor.Contract(descriptor.Fields{}, scalarVocabulary())

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
			Details:   map[string]string{"key": "name", "type": "string"},
		},
	}, violations)
}

func TestContract_PropertyKeyAddedByAVocabulary_NotTakenOutsideAProperty(t *testing.T) {
	grammar := descriptor.Contract(descriptor.Fields{}, scalarVocabulary())

	violations := descriptor.Validate(grammar, map[string]any{"schemas": map[string]any{
		"Pets": map[string]any{"type": "array", "items": map[string]any{"type": "string", "arguments": "PetsArguments"}},
	}}, "api.yaml")

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;Pets;items;arguments",
			Source:    "api.yaml",
			Details:   map[string]string{"key": "arguments"},
		},
	}, violations)
}
