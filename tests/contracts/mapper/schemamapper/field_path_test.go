package schemamapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestToSchemaModels_TheSchemaItself_IsTheEmptyPath(t *testing.T) {
	schemas := schemamapper.ToSchemaModels(contract.SchemasMap{"Name": {Type: "string"}}, []string{"Name"})

	assert.Equal(t, model.Schemas{
		"Name": {"": {Type: "string"}},
	}, schemas)
}

func TestToSchemaModels_PropertyOfTheSchema_IsItsBareName(t *testing.T) {
	catalog := contract.SchemasMap{
		"Address": {
			Type:       "object",
			Properties: map[string]contract.Schema{"city": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Address"})

	assert.Equal(t, model.Schemas{
		"Address": {
			"":     {Type: "object"},
			"city": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_PropertyOfAnInlineObject_JoinsTheNamesWithADot(t *testing.T) {
	catalog := contract.SchemasMap{
		"Order": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"meta": {
					Type:       "object",
					Properties: map[string]contract.Schema{"x": {Type: "integer"}},
				},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Order"})

	assert.Equal(t, model.Schemas{
		"Order": {
			"":       {Type: "object"},
			"meta":   {Type: "object"},
			"meta.x": {Type: "integer"},
		},
	}, schemas)
}

func TestToSchemaModels_ArrayItems_SuffixBrackets(t *testing.T) {
	catalog := contract.SchemasMap{
		"Order": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"items": {
					Type: "array",
					Items: &contract.Schema{
						Type:       "object",
						Properties: map[string]contract.Schema{"sku": {Type: "string"}},
					},
				},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Order"})

	assert.Equal(t, model.Schemas{
		"Order": {
			"":            {Type: "object"},
			"items":       {Type: "array"},
			"items[]":     {Type: "object"},
			"items[].sku": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_ItemsOfASchemaThatIsAnArray_StartWithBrackets(t *testing.T) {
	catalog := contract.SchemasMap{
		"PetList": {
			Type: "array",
			Items: &contract.Schema{
				Type:       "object",
				Properties: map[string]contract.Schema{"id": {Type: "string"}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"PetList"})

	assert.Equal(t, model.Schemas{
		"PetList": {
			"":      {Type: "array"},
			"[]":    {Type: "object"},
			"[].id": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_ItemsOfNestedArrays_StackBrackets(t *testing.T) {
	catalog := contract.SchemasMap{
		"Matrix": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"rows": {Type: "array", Items: &contract.Schema{Type: "array", Items: &contract.Schema{Type: "number"}}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Matrix"})

	assert.Equal(t, model.Schemas{
		"Matrix": {
			"":         {Type: "object"},
			"rows":     {Type: "array"},
			"rows[]":   {Type: "array"},
			"rows[][]": {Type: "number"},
		},
	}, schemas)
}

func TestToSchemaModels_MapValuesOfAProperty_SuffixBraces(t *testing.T) {
	catalog := contract.SchemasMap{
		"Report": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"scores": {Type: "object", Values: &contract.Schema{Type: "integer"}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Report"})

	assert.Equal(t, model.Schemas{
		"Report": {
			"":         {Type: "object"},
			"scores":   {Type: "object"},
			"scores{}": {Type: "integer"},
		},
	}, schemas)
}

func TestToSchemaModels_ValuesOfASchemaThatIsAMap_StartWithBraces(t *testing.T) {
	catalog := contract.SchemasMap{
		"Pets": {
			Type: "object",
			Values: &contract.Schema{
				Type:       "object",
				Properties: map[string]contract.Schema{"name": {Type: "string"}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Pets"})

	assert.Equal(t, model.Schemas{
		"Pets": {
			"":        {Type: "object"},
			"{}":      {Type: "object"},
			"{}.name": {Type: "string"},
		},
	}, schemas)
}
