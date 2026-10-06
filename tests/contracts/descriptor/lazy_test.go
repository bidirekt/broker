package descriptor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

func tree() descriptor.Node {
	return descriptor.Object(descriptor.Fields{
		"name":     descriptor.String(),
		"children": descriptor.Map(descriptor.Key{}, descriptor.Lazy(tree)),
	})
}

func TestLazy_ResolveAGramaticaNaValidacao(t *testing.T) {
	document := map[string]any{
		"name": "root",
		"children": map[string]any{
			"left": map[string]any{
				"name": "left",
				"children": map[string]any{
					"leaf": map[string]any{"name": 1},
				},
			},
		},
	}

	violations := descriptor.Validate(tree(), document, "tree.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "children;left;children;leaf;name", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "string", "got": "integer"}, violations[0].Details)
}

func TestLazy_AceitaProfundidadeArbitraria(t *testing.T) {
	document := map[string]any{
		"children": map[string]any{
			"a": map[string]any{
				"children": map[string]any{
					"b": map[string]any{
						"children": map[string]any{
							"c": map[string]any{"name": "c"},
						},
					},
				},
			},
		},
	}

	violations := descriptor.Validate(tree(), document, "tree.yaml")

	assert.Empty(t, violations)
}
