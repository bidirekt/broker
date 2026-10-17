package schemagraph_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

func graphRoot(catalog model.Schemas, schemaName string) schemagraph.Node {
	return schemagraph.New(catalog).Root(
		[]model.ResourceBody{{Source: "orders.yaml", SchemaName: null.StringFrom(schemaName)}},
		schemagraph.Senders,
	)
}

func TestRef_ChainOfRefs_ResolvesToTheLastFieldAndOrsOptionalAndNullable(t *testing.T) {
	catalog := model.Schemas{
		"Order": {
			"":         {Type: "object"},
			"customer": {Ref: "Customer", Optional: true},
		},
		"Customer": {"": {Ref: "Person"}},
		"Person": {
			"":     {Type: "object", Nullable: true},
			"name": {Type: "string"},
		},
	}

	customer := graphRoot(catalog, "Order").Child("customer")

	assert.True(t, customer.Exists())
	assert.Equal(t, "object", customer.Type())
	assert.True(t, customer.Optional())
	assert.True(t, customer.Nullable())
	assert.Equal(t, []string{"name"}, customer.Segments())
	assert.Equal(t, "string", customer.Child("name").Type())
}

func TestRef_WithoutFlags_LeavesTheTargetFlags(t *testing.T) {
	catalog := model.Schemas{
		"Order": {
			"":      {Type: "object"},
			"total": {Ref: "Money"},
		},
		"Money": {"": {Type: "float"}},
	}

	total := graphRoot(catalog, "Order").Child("total")

	assert.Equal(t, "float", total.Type())
	assert.False(t, total.Optional())
	assert.False(t, total.Nullable())
}

func TestRef_PureRefCycle_ResolvesToANodeWithoutTypeOrChildren(t *testing.T) {
	cases := map[string]model.Schemas{
		"to itself": {"Loop": {"": {Ref: "Loop"}}},
		"through another schema": {
			"Loop": {"": {Ref: "Echo"}},
			"Echo": {"": {Ref: "Loop"}},
		},
	}

	for name, catalog := range cases {
		t.Run(name, func(t *testing.T) {
			loop := graphRoot(catalog, "Loop")

			assert.True(t, loop.Exists())
			assert.Empty(t, loop.Type())
			assert.Empty(t, loop.Segments())
		})
	}
}

func TestRef_ToAnUndeclaredSchema_ResolvesToAnEmptyNode(t *testing.T) {
	catalog := model.Schemas{
		"Order": {
			"":      {Type: "object"},
			"ghost": {Ref: "Ghost", Nullable: true},
		},
	}

	ghost := graphRoot(catalog, "Order").Child("ghost")

	assert.True(t, ghost.Exists())
	assert.Empty(t, ghost.Type())
	assert.True(t, ghost.Nullable())
	assert.Empty(t, ghost.Segments())
}

func TestRoot_UndeclaredSchema_IsAnEmptyNode(t *testing.T) {
	root := graphRoot(model.Schemas{}, "Ghost")

	assert.True(t, root.Exists())
	assert.Empty(t, root.Type())
	assert.Empty(t, root.Segments())
}

func TestRoot_UndescribedBody_DoesNotExist(t *testing.T) {
	root := schemagraph.New(model.Schemas{}).Root([]model.ResourceBody{{Source: "files.yaml"}}, schemagraph.Readers)

	assert.False(t, root.Exists())
	assert.Empty(t, root.Type())
}

func TestChild_NotDeclared_DoesNotExist(t *testing.T) {
	catalog := model.Schemas{"Order": {"": {Type: "object"}, "id": {Type: "string"}}}

	assert.False(t, graphRoot(catalog, "Order").Child("total").Exists())
	assert.False(t, graphRoot(catalog, "Order").Child("id").Child("value").Exists())
}

func TestSegments_ListPropertiesItemsAndValuesInByteOrder(t *testing.T) {
	catalog := model.Schemas{
		"Order": {
			"":            {Type: "object"},
			"total":       {Type: "float"},
			"Currency":    {Type: "string"},
			"_links":      {Type: "object"},
			"_links.self": {Type: "string"},
			"items":       {Type: "array"},
			"items[]":     {Type: "object"},
			"items[].sku": {Type: "string"},
			"meta":        {Type: "map"},
			"meta{}":      {Type: "string"},
			"rows":        {Type: "array"},
			"rows[]":      {Type: "array"},
			"rows[][]":    {Type: "integer"},
		},
	}

	root := graphRoot(catalog, "Order")

	assert.Equal(t, []string{"Currency", "_links", "items", "meta", "rows", "total"}, root.Segments())
	assert.Equal(t, []string{"self"}, root.Child("_links").Segments())
	assert.Equal(t, []string{"[]"}, root.Child("items").Segments())
	assert.Equal(t, []string{"sku"}, root.Child("items").Child("[]").Segments())
	assert.Equal(t, []string{"{}"}, root.Child("meta").Segments())
	assert.Equal(t, "integer", root.Child("rows").Child("[]").Child("[]").Type())
}

func TestSegments_OfARootArrayAndARootMap(t *testing.T) {
	catalog := model.Schemas{
		"Tags": {"": {Type: "array"}, "[]": {Type: "string"}},
		"Pets": {"": {Type: "map"}, "{}": {Type: "object"}, "{}.name": {Type: "string"}},
	}

	assert.Equal(t, []string{"[]"}, graphRoot(catalog, "Tags").Segments())
	assert.Equal(t, "string", graphRoot(catalog, "Tags").Child("[]").Type())
	assert.Equal(t, []string{"{}"}, graphRoot(catalog, "Pets").Segments())
	assert.Equal(t, []string{"name"}, graphRoot(catalog, "Pets").Child("{}").Segments())
}

func TestSegments_OfARecursiveSchema_FollowTheRefBackToTheSchema(t *testing.T) {
	catalog := model.Schemas{
		"Comment": {
			"":          {Type: "object"},
			"text":      {Type: "string"},
			"replies":   {Type: "array"},
			"replies[]": {Ref: "Comment"},
		},
	}

	reply := graphRoot(catalog, "Comment").Child("replies").Child("[]")

	assert.Equal(t, "object", reply.Type())
	assert.Equal(t, []string{"replies", "text"}, reply.Segments())
	assert.Equal(t, graphRoot(catalog, "Comment").ShapeKey(), reply.ShapeKey())
	assert.NotEqual(t, graphRoot(catalog, "Comment").SiteKey(), reply.SiteKey())
}

func TestField_NamesTheSchemaAndThePathTheNodeWasReachedAt(t *testing.T) {
	catalog := model.Schemas{
		"Order": {
			"":            {Type: "object"},
			"billing":     {Ref: "Address"},
			"items":       {Type: "array"},
			"items[]":     {Type: "object"},
			"items[].sku": {Type: "string"},
		},
		"Address": {"": {Type: "object"}, "city": {Type: "string"}},
		"Pets":    {"": {Type: "map"}, "{}": {Type: "object"}, "{}.name": {Type: "string"}},
		"Tags":    {"": {Type: "array"}, "[]": {Type: "string"}},
	}

	order := graphRoot(catalog, "Order")

	assert.Equal(t, "Order", order.Field())
	assert.Equal(t, "Order.billing", order.Child("billing").Field())
	assert.Equal(t, "Address.city", order.Child("billing").Child("city").Field())
	assert.Equal(t, "Order.items[]", order.Child("items").Child("[]").Field())
	assert.Equal(t, "Order.items[].sku", order.Child("items").Child("[]").Child("sku").Field())
	assert.Equal(t, "Pets{}.name", graphRoot(catalog, "Pets").Child("{}").Child("name").Field())
	assert.Equal(t, "Tags[]", graphRoot(catalog, "Tags").Child("[]").Field())
	assert.Empty(t, order.Child("total").Field())
}
