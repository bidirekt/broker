package descriptor_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func listDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestList_EveryItemFollowsTheItemNode(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{"values": descriptor.List(descriptor.String())})

	violations := descriptor.Validate(subject, listDocument(t, "values: [a, b, c]\n"), "api.yaml")

	assert.Empty(t, violations)
}

func TestList_ItemPathIsItsIndexStartingAtZero(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{"values": descriptor.List(descriptor.String())})

	violations := descriptor.Validate(subject, listDocument(t, "values: [{}, b, [x]]\n"), "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "values;0", violations[0].Path)
	assert.Equal(t, "api.yaml", violations[0].Source)
	assert.Equal(t, map[string]string{"expected": "string", "got": "mapping"}, violations[0].Details)
	assert.Equal(t, "values;2", violations[1].Path)
	assert.Equal(t, map[string]string{"expected": "string", "got": "sequence"}, violations[1].Details)
}

func TestList_ItemsKeepNestedPaths(t *testing.T) {
	subject := descriptor.List(descriptor.Object(descriptor.Fields{"name": descriptor.String()}))

	violations := descriptor.Validate(subject, listDocument(t, "- name: a\n- extra: b\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "key.unknown", violations[0].ErrorCode)
	assert.Equal(t, "1;extra", violations[0].Path)
}

func TestList_ValueThatIsNotASequenceIsAnInvalidKind(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{"values": descriptor.List(descriptor.String())})

	violations := descriptor.Validate(subject, listDocument(t, "values:\n  a: b\n"), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "values", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "sequence", "got": "mapping"}, violations[0].Details)
}

func TestList_NullAndEmptyListPass(t *testing.T) {
	subject := descriptor.Object(descriptor.Fields{"values": descriptor.List(descriptor.String())})

	assert.Empty(t, descriptor.Validate(subject, listDocument(t, "values: ~\n"), "api.yaml"))
	assert.Empty(t, descriptor.Validate(subject, listDocument(t, "values: []\n"), "api.yaml"))
}
