package schemamapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addressSchemas(targetOptional bool) contract.SchemasMap {
	return contract.SchemasMap{
		"Address": {
			Type:     "object",
			Optional: targetOptional,
			Properties: map[string]contract.Schema{
				"street": {Type: "string"},
			},
		},
	}
}

func userWithAddress(siteOptional bool) contract.Schema {
	return contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"address": {Ref: "Address", Optional: siteOptional},
		},
	}
}

func TestToPropertyModels_OptionalRefSite_MakesTheNodeOptional(t *testing.T) {
	properties := schemamapper.ToPropertyModels(addressSchemas(false), userWithAddress(true))

	require.Contains(t, properties, "$.address")
	assert.Equal(t, model.Property{Path: "$.address", Type: "object", Optional: true}, properties["$.address"])
	assert.Equal(t, model.Property{Path: "$.address.street", Type: "string", Optional: false}, properties["$.address.street"])
}

func TestToPropertyModels_RequiredRefSite_ToRequiredTarget_StaysRequired(t *testing.T) {
	properties := schemamapper.ToPropertyModels(addressSchemas(false), userWithAddress(false))

	require.Contains(t, properties, "$.address")
	assert.Equal(t, model.Property{Path: "$.address", Type: "object", Optional: false}, properties["$.address"])
}

func TestToPropertyModels_RequiredRefSite_ToOptionalTarget_StaysOptional(t *testing.T) {
	properties := schemamapper.ToPropertyModels(addressSchemas(true), userWithAddress(false))

	require.Contains(t, properties, "$.address")
	assert.Equal(t, model.Property{Path: "$.address", Type: "object", Optional: true}, properties["$.address"])
}

func TestToPropertyModels_NumberProperty_KeepsTheNumberType(t *testing.T) {
	priced := contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"price": {Type: "number"},
		},
	}

	properties := schemamapper.ToPropertyModels(contract.SchemasMap{}, priced)

	assert.Equal(t, model.Property{Path: "$.price", Type: "number"}, properties["$.price"])
}

func TestToPropertyModels_NullableOnEveryNode_FlattensNullable(t *testing.T) {
	root := contract.Schema{
		Type:     "object",
		Nullable: true,
		Properties: map[string]contract.Schema{
			"avatar":   {Type: "string", Nullable: true},
			"nickname": {Type: "string", Optional: true},
			"bio":      {Type: "string", Optional: true, Nullable: true},
			"tags":     {Type: "array", Items: &contract.Schema{Type: "string", Nullable: true}},
		},
	}

	properties := schemamapper.ToPropertyModels(contract.SchemasMap{}, root)

	assert.Equal(t, model.Property{Path: "$", Type: "object", Nullable: true}, properties["$"])
	assert.Equal(t, model.Property{Path: "$.avatar", Type: "string", Nullable: true}, properties["$.avatar"])
	assert.Equal(t, model.Property{Path: "$.nickname", Type: "string", Optional: true}, properties["$.nickname"])
	assert.Equal(t, model.Property{Path: "$.bio", Type: "string", Optional: true, Nullable: true}, properties["$.bio"])
	assert.Equal(t, model.Property{Path: "$.tags", Type: "array"}, properties["$.tags"])
	assert.Equal(t, model.Property{Path: "$.tags[]", Type: "string", Nullable: true}, properties["$.tags[]"])
}

func TestToPropertyModels_NullableRef_OrsWithTheTarget(t *testing.T) {
	schemas := contract.SchemasMap{
		"Manager":         {Type: "object", Properties: map[string]contract.Schema{"name": {Type: "string"}}},
		"NullableManager": {Type: "object", Nullable: true, Properties: map[string]contract.Schema{"name": {Type: "string"}}},
	}
	root := contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"nullableSite":   {Ref: "Manager", Nullable: true},
			"nullableTarget": {Ref: "NullableManager"},
			"neither":        {Ref: "Manager"},
		},
	}

	properties := schemamapper.ToPropertyModels(schemas, root)

	assert.Equal(t, model.Property{Path: "$.nullableSite", Type: "object", Nullable: true}, properties["$.nullableSite"])
	assert.Equal(t, model.Property{Path: "$.nullableSite.name", Type: "string"}, properties["$.nullableSite.name"])
	assert.Equal(t, model.Property{Path: "$.nullableTarget", Type: "object", Nullable: true}, properties["$.nullableTarget"])
	assert.Equal(t, model.Property{Path: "$.neither", Type: "object"}, properties["$.neither"])
}
