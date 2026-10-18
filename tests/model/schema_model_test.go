package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

var schemaPathSpellings = []struct {
	parent, segment, path string
}{
	{"", "city", "city"},
	{"meta", "x", "meta.x"},
	{"", model.ItemsSegment, "[]"},
	{"items", model.ItemsSegment, "items[]"},
	{"items[]", "sku", "items[].sku"},
	{"rows[]", model.ItemsSegment, "rows[][]"},
	{"", model.ValuesSegment, "{}"},
	{"pets", model.ValuesSegment, "pets{}"},
	{"{}", "name", "{}.name"},
	{"tags{}", model.ItemsSegment, "tags{}[]"},
	{"$", "name", "$.name"},
	{"$", model.ItemsSegment, "$[]"},
}

func TestChildPath_SpellsPropertiesItemsAndValues(t *testing.T) {
	for _, tc := range schemaPathSpellings {
		assert.Equal(t, tc.path, model.ChildPath(tc.parent, tc.segment), tc.parent+" + "+tc.segment)
	}
}

func TestParentOf_SplitsWhatChildPathJoined(t *testing.T) {
	for _, tc := range schemaPathSpellings {
		parent, segment, nested := model.ParentOf(tc.path)

		assert.True(t, nested, tc.path)
		assert.Equal(t, tc.parent, parent, tc.path)
		assert.Equal(t, tc.segment, segment, tc.path)
	}
}

func TestParentOf_TheSchemaItself_HasNoParent(t *testing.T) {
	_, _, nested := model.ParentOf("")

	assert.False(t, nested)
}

func TestFieldName_SpellsTheSchemaFollowedByThePathInsideIt(t *testing.T) {
	fieldNames := []struct {
		schema, path, field string
	}{
		{"Order", "", "Order"},
		{"Order", "billing", "Order.billing"},
		{"Order", "items[].sku", "Order.items[].sku"},
		{"Tags", "[]", "Tags[]"},
		{"Pets", "{}.name", "Pets{}.name"},
		{"PriceTables", "[]{}", "PriceTables[]{}"},
	}

	for _, tc := range fieldNames {
		assert.Equal(t, tc.field, model.FieldName(tc.schema, tc.path), tc.schema+" + "+tc.path)
	}
}
