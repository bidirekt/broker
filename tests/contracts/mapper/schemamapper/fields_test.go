package schemamapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestToSchemaModels_DeclaredType_IsTheFieldType(t *testing.T) {
	for _, declaredType := range []string{"object", "string", "integer", "float", "number", "boolean"} {
		t.Run(declaredType, func(t *testing.T) {
			schemas := schemamapper.ToSchemaModels(contract.SchemasMap{"Value": {Type: declaredType}}, []string{"Value"})

			assert.Equal(t, model.Schemas{
				"Value": {"": {Type: declaredType}},
			}, schemas)
		})
	}
}

func TestToSchemaModels_PropertiesWithoutType_IsNotAnObject(t *testing.T) {
	catalog := contract.SchemasMap{
		"Address": {Properties: map[string]contract.Schema{"city": {Type: "string"}}},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Address"})

	assert.Equal(t, model.Schemas{"Address": {"": {}}}, schemas)
}

func TestToSchemaModels_ValuesWithoutType_IsNotAnObject(t *testing.T) {
	catalog := contract.SchemasMap{
		"Scores": {Values: &contract.Schema{Type: "integer"}},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Scores"})

	assert.Equal(t, model.Schemas{"Scores": {"": {}}}, schemas)
}

func TestToSchemaModels_ItemsWithoutType_IsNotAnArray(t *testing.T) {
	catalog := contract.SchemasMap{
		"Tags": {Items: &contract.Schema{Type: "string"}},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Tags"})

	assert.Equal(t, model.Schemas{"Tags": {"": {}}}, schemas)
}

func TestToSchemaModels_RefField_KeepsTheTargetNameAndNothingBelowIt(t *testing.T) {
	catalog := contract.SchemasMap{
		"User": {
			Type:       "object",
			Properties: map[string]contract.Schema{"address": {Ref: "Address"}},
		},
		"Address": {
			Type:       "object",
			Properties: map[string]contract.Schema{"street": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"User"})

	assert.Equal(t, model.Schemas{
		"User": {
			"":        {Type: "object"},
			"address": {Ref: "Address"},
		},
		"Address": {
			"":       {Type: "object"},
			"street": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_SchemaThatIsARef_IsARefFieldItself(t *testing.T) {
	catalog := contract.SchemasMap{
		"PetRef": {Ref: "Pet"},
		"Pet": {
			Type:       "object",
			Properties: map[string]contract.Schema{"id": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"PetRef"})

	assert.Equal(t, model.Schemas{
		"PetRef": {"": {Ref: "Pet"}},
		"Pet": {
			"":   {Type: "object"},
			"id": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_RefAsArrayItemsAndAsMapValues_IsARefField(t *testing.T) {
	catalog := contract.SchemasMap{
		"Shelter": {
			Type: "object",
			Properties: map[string]contract.Schema{
				"queue":  {Type: "array", Items: &contract.Schema{Ref: "Pet"}},
				"byName": {Type: "object", Values: &contract.Schema{Ref: "Pet"}},
			},
		},
		"Pet": {
			Type:       "object",
			Properties: map[string]contract.Schema{"name": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Shelter"})

	assert.Equal(t, model.Schemas{
		"Shelter": {
			"":         {Type: "object"},
			"queue":    {Type: "array"},
			"queue[]":  {Ref: "Pet"},
			"byName":   {Type: "object"},
			"byName{}": {Ref: "Pet"},
		},
		"Pet": {
			"":     {Type: "object"},
			"name": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_TypeWrittenNextToARef_IgnoresTheRef(t *testing.T) {
	catalog := contract.SchemasMap{
		"Contact": {
			Type:       "object",
			Properties: map[string]contract.Schema{"address": {Type: "string", Ref: "Address"}},
		},
		"Address": {
			Type:       "object",
			Properties: map[string]contract.Schema{"city": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Contact"})

	assert.Equal(t, model.Schemas{
		"Contact": {
			"":        {Type: "object"},
			"address": {Type: "string"},
		},
	}, schemas)
}

func TestToSchemaModels_OptionalAndNullableRefSite_StayOnTheSiteField(t *testing.T) {
	catalog := contract.SchemasMap{
		"User": {
			Type:       "object",
			Properties: map[string]contract.Schema{"address": {Ref: "Address", Optional: true, Nullable: true}},
		},
		"Address": {
			Type:       "object",
			Properties: map[string]contract.Schema{"street": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"User"})

	assert.Equal(t, model.Field{Ref: "Address", Optional: true, Nullable: true}, schemas["User"]["address"])
	assert.Equal(t, model.Field{Type: "object"}, schemas["Address"][""])
}

func TestToSchemaModels_OptionalAndNullableTarget_StayOnTheTargetSchema(t *testing.T) {
	catalog := contract.SchemasMap{
		"User": {
			Type:       "object",
			Properties: map[string]contract.Schema{"manager": {Ref: "Manager"}},
		},
		"Manager": {
			Type:       "object",
			Optional:   true,
			Nullable:   true,
			Properties: map[string]contract.Schema{"name": {Type: "string"}},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"User"})

	assert.Equal(t, model.Field{Ref: "Manager"}, schemas["User"]["manager"])
	assert.Equal(t, model.Field{Type: "object", Optional: true, Nullable: true}, schemas["Manager"][""])
}

func TestToSchemaModels_OptionalAndNullable_StayOnTheNodeThatDeclaresThem(t *testing.T) {
	catalog := contract.SchemasMap{
		"Profile": {
			Type:     "object",
			Nullable: true,
			Properties: map[string]contract.Schema{
				"avatar":   {Type: "string", Nullable: true},
				"nickname": {Type: "string", Optional: true},
				"bio":      {Type: "string", Optional: true, Nullable: true},
				"tags":     {Type: "array", Items: &contract.Schema{Type: "string", Nullable: true}},
				"scores":   {Type: "object", Values: &contract.Schema{Type: "integer", Optional: true, Nullable: true}},
			},
		},
	}

	schemas := schemamapper.ToSchemaModels(catalog, []string{"Profile"})

	assert.Equal(t, model.Schemas{
		"Profile": {
			"":         {Type: "object", Nullable: true},
			"avatar":   {Type: "string", Nullable: true},
			"nickname": {Type: "string", Optional: true},
			"bio":      {Type: "string", Optional: true, Nullable: true},
			"tags":     {Type: "array"},
			"tags[]":   {Type: "string", Nullable: true},
			"scores":   {Type: "object"},
			"scores{}": {Type: "integer", Optional: true, Nullable: true},
		},
	}, schemas)
}
