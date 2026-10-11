package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestProperty_IsSame_Identical(t *testing.T) {
	a := model.NewProperty("$.id", "string", false)
	b := model.NewProperty("$.id", "string", false)

	assert.True(t, a.IsSame(&b))
}

func TestProperty_IsSame_FalseOnTypeMismatch(t *testing.T) {
	a := model.NewProperty("$.id", "string", false)
	b := model.NewProperty("$.id", "int", false)

	assert.False(t, a.IsSame(&b))
}

func TestParentPath_StripsTheLastSegment(t *testing.T) {
	cases := []struct {
		path   string
		parent string
	}{
		{"$.name", "$"},
		{"$.address.city", "$.address"},
		{"$.categories[]", "$.categories"},
		{"$.pets{}", "$.pets"},
		{"$.list[].name", "$.list[]"},
		{"$.tags[][]", "$.tags[]"},
		{"$.violations{}.message", "$.violations{}"},
		{"$[]", "$"},
		{"${}", "$"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			parent, hasParent := model.ParentPath(tc.path)

			assert.True(t, hasParent)
			assert.Equal(t, tc.parent, parent)
		})
	}
}

func TestParentPath_RootHasNoParent(t *testing.T) {
	_, hasParent := model.ParentPath("$")

	assert.False(t, hasParent)
}
