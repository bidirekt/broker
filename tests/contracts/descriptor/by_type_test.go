package descriptor_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func byTypeDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func byTypeGrammar() descriptor.Node {
	return descriptor.Object(descriptor.Fields{
		"type": descriptor.String(),
		"members": descriptor.ByType{
			"enum":  descriptor.List(descriptor.String()),
			"union": descriptor.Map(descriptor.Key{}, descriptor.Object(descriptor.Fields{"ref": descriptor.String()})),
		},
	})
}

func TestByType_SiblingTypeChoosesTheNode(t *testing.T) {
	enum := descriptor.Validate(byTypeGrammar(), byTypeDocument(t, "type: enum\nmembers: [a, b]\n"), "api.yaml")
	union := descriptor.Validate(byTypeGrammar(), byTypeDocument(t, "type: union\nmembers:\n  card: { ref: Card }\n"), "api.yaml")

	assert.Empty(t, enum)
	assert.Empty(t, union)
}

func TestByType_ChosenNodeReportsItsOwnViolations(t *testing.T) {
	enum := descriptor.Validate(byTypeGrammar(), byTypeDocument(t, "type: enum\nmembers:\n  card: { ref: Card }\n"), "api.yaml")
	union := descriptor.Validate(byTypeGrammar(), byTypeDocument(t, "type: union\nmembers:\n  card: { type: Card }\n"), "api.yaml")

	require.Len(t, enum, 1)
	assert.Equal(t, "value.invalid_kind", enum[0].ErrorCode)
	assert.Equal(t, "members", enum[0].Path)
	assert.Equal(t, map[string]string{"expected": "sequence", "got": "mapping"}, enum[0].Details)

	require.Len(t, union, 1)
	assert.Equal(t, "key.unknown", union[0].ErrorCode)
	assert.Equal(t, "members;card;type", union[0].Path)
}

func TestByType_UnlistedOrMissingTypeLeavesTheFieldUnvalidated(t *testing.T) {
	unlisted := descriptor.Validate(byTypeGrammar(), byTypeDocument(t, "type: string\nmembers: 42\n"), "api.yaml")
	missing := descriptor.Validate(byTypeGrammar(), byTypeDocument(t, "members: 42\n"), "api.yaml")

	assert.Empty(t, unlisted)
	assert.Empty(t, missing)
}
