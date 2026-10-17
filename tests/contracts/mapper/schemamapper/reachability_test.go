package schemamapper_test

import (
	"fmt"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func levelName(level int) string {
	return fmt.Sprintf("S%d", level)
}

func countFields(schemas model.Schemas) int {
	count := 0
	for _, schema := range schemas {
		count += len(schema)
	}

	return count
}

func TestToSchemaModels_TwentyTwoLevelsWithTwoRefsPerLevel_KeepsOnlyTheDeclaredFields(t *testing.T) {
	const levels = 22

	catalog := contract.SchemasMap{levelName(levels): {Type: "string"}}
	want := model.Schemas{levelName(levels): {"": {Type: "string"}}}

	for level := range levels {
		next := levelName(level + 1)

		catalog[levelName(level)] = contract.Schema{
			Type: "object",
			Properties: map[string]contract.Schema{
				"left":  {Ref: next},
				"right": {Ref: next},
			},
		}
		want[levelName(level)] = model.Schema{
			"":      {Type: "object"},
			"left":  {Ref: next},
			"right": {Ref: next},
		}
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{levelName(0)})

	assert.Equal(t, want, schemas)
	assert.Len(t, schemas, 23)
	assert.Equal(t, 67, countFields(schemas))
}

func TestToSchemaModels_OnlySchemasReachableFromTheRoots_AreConverted(t *testing.T) {
	catalog := contract.SchemasMap{
		"Order": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"lines": {Type: "array", Items: &contract.Schema{Ref: "Line"}},
			},
		},
		"Line": {
			Type:       "object",
			Properties: map[string]contract.Schema{"price": {Ref: "Money"}},
		},
		"Money": {
			Type:       "object",
			Properties: map[string]contract.Schema{"amount": {Type: "number"}},
		},
		"Coupon": {
			Type:       "object",
			Properties: map[string]contract.Schema{"code": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Order"})

	assert.Equal(t, model.Schemas{
		"Order": {
			"":        {Type: "object"},
			"lines":   {Type: "array"},
			"lines[]": {Ref: "Line"},
		},
		"Line": {
			"":      {Type: "object"},
			"price": {Ref: "Money"},
		},
		"Money": {
			"":       {Type: "object"},
			"amount": {Type: "number"},
		},
	}, schemas)
}

func TestToSchemaModels_RootsSharingASchema_ConvertItOnce(t *testing.T) {
	catalog := contract.SchemasMap{
		"Pet": {
			Type:       "object",
			Properties: map[string]contract.Schema{"id": {Type: "string"}},
		},
		"PetList": {Type: "array", Items: &contract.Schema{Ref: "Pet"}},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Pet", "PetList", "Pet"})

	assert.Equal(t, model.Schemas{
		"Pet": {
			"":   {Type: "object"},
			"id": {Type: "string"},
		},
		"PetList": {
			"":   {Type: "array"},
			"[]": {Ref: "Pet"},
		},
	}, schemas)
}

func TestToSchemaModels_NoRoots_ConvertsNothing(t *testing.T) {
	catalog := contract.SchemasMap{"Pet": {Type: "string"}}

	assert.Equal(t, model.Schemas{}, schemamapper.ToSchemaModels(catalog, nil))
}

func TestToSchemaModels_UndeclaredRoot_IsSkipped(t *testing.T) {
	catalog := contract.SchemasMap{"Pet": {Type: "string"}}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Ghost", "Pet"})

	assert.Equal(t, model.Schemas{
		"Pet": {"": {Type: "string"}},
	}, schemas)
}

func TestToSchemaModels_RefToAnUndeclaredName_IsKeptButNotFollowed(t *testing.T) {
	catalog := contract.SchemasMap{
		"Pet": {
			Type:       "object",
			Properties: map[string]contract.Schema{"owner": {Ref: "Ghost"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Pet"})

	assert.Equal(t, model.Schemas{
		"Pet": {
			"":      {Type: "object"},
			"owner": {Ref: "Ghost"},
		},
	}, schemas)
}

func TestToSchemaModels_SchemaReferencingItselfThroughArrayItems_IsConvertedOnce(t *testing.T) {
	catalog := contract.SchemasMap{
		"Comment": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"text":    {Type: "string"},
				"replies": {Type: "array", Items: &contract.Schema{Ref: "Comment"}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Comment"})

	assert.Equal(t, model.Schemas{
		"Comment": {
			"":          {Type: "object"},
			"text":      {Type: "string"},
			"replies":   {Type: "array"},
			"replies[]": {Ref: "Comment"},
		},
	}, schemas)
}

func TestToSchemaModels_SchemaReferencingItselfThroughMapValues_IsConvertedOnce(t *testing.T) {
	catalog := contract.SchemasMap{
		"Node": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"children": {Type: "map", Values: &contract.Schema{Ref: "Node"}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Node"})

	assert.Equal(t, model.Schemas{
		"Node": {
			"":           {Type: "object"},
			"children":   {Type: "map"},
			"children{}": {Ref: "Node"},
		},
	}, schemas)
}

func TestToSchemaModels_SchemasReferencingEachOther_AreEachConvertedOnce(t *testing.T) {
	catalog := contract.SchemasMap{
		"Post": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"title":  {Type: "string"},
				"author": {Ref: "User"},
			},
		},
		"User": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"name":  {Type: "string"},
				"posts": {Type: "array", Items: &contract.Schema{Ref: "Post"}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Post"})

	assert.Equal(t, model.Schemas{
		"Post": {
			"":       {Type: "object"},
			"title":  {Type: "string"},
			"author": {Ref: "User"},
		},
		"User": {
			"":        {Type: "object"},
			"name":    {Type: "string"},
			"posts":   {Type: "array"},
			"posts[]": {Ref: "Post"},
		},
	}, schemas)
}

func TestToSchemaModels_SchemaThatIsARefToItself_IsConvertedOnce(t *testing.T) {
	catalog := contract.SchemasMap{"Loop": {Ref: "Loop"}}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Loop"})

	assert.Equal(t, model.Schemas{
		"Loop": {"": {Ref: "Loop"}},
	}, schemas)
}
