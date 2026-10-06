package contract_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func schemaDocument(t *testing.T, source string) contract.Document {
	t.Helper()

	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestSchemaFromDocument_Primitivo(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: string
description: nome do pet
`))

	assert.Equal(t, contract.Schema{Type: "string", Description: "nome do pet"}, schema)
	assert.True(t, schema.IsPrimitive())
	assert.False(t, schema.IsObject())
	assert.False(t, schema.IsArray())
	assert.False(t, schema.IsRef())
}

func TestSchemaFromDocument_CadaTipoPrimitivo(t *testing.T) {
	for _, primitive := range []string{"string", "integer", "float", "boolean"} {
		t.Run(primitive, func(t *testing.T) {
			schema := contract.SchemaFromDocument(contract.Document{"type": primitive})

			assert.True(t, schema.IsPrimitive())
		})
	}
}

func TestSchemaFromDocument_TipoDesconhecidoNaoEhNada(t *testing.T) {
	schema := contract.SchemaFromDocument(contract.Document{"type": "auid"})

	assert.Equal(t, contract.Schema{Type: "auid"}, schema)
	assert.False(t, schema.IsPrimitive())
	assert.False(t, schema.IsObject())
	assert.False(t, schema.IsArray())
	assert.False(t, schema.IsRef())
}

func TestSchemaFromDocument_TypeQueNaoEhStringEhIgnorado(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: 42
`))

	assert.Equal(t, contract.Schema{}, schema)
}

func TestSchemaFromDocument_ObjetoComProperties(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: object
properties:
  id:
    type: string
  name:
    type: string
    optional: true
`))

	assert.Equal(t, contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"id":   {Type: "string"},
			"name": {Type: "string", Optional: true},
		},
	}, schema)
	assert.True(t, schema.IsObject())
	assert.False(t, schema.IsArray())
	assert.False(t, schema.IsPrimitive())
	assert.False(t, schema.IsRef())
}

func TestSchemaFromDocument_ObjetoSemTypeMasComPropertiesEhObjeto(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `properties: {}
`))

	assert.NotNil(t, schema.Properties)
	assert.Empty(t, schema.Properties)
	assert.True(t, schema.IsObject())
}

func TestSchemaFromDocument_PropriedadeQueNaoEhMapaViraSchemaZero(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: object
properties:
  id: string
`))

	assert.Equal(t, map[string]contract.Schema{"id": {}}, schema.Properties)
}

func TestSchemaFromDocument_PropertiesQueNaoEhMapaEhIgnorado(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `properties: id
`))

	assert.Nil(t, schema.Properties)
	assert.False(t, schema.IsObject())
}

func TestSchemaFromDocument_ArrayComItems(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: array
items:
  type: integer
`))

	assert.Equal(t, contract.Schema{Type: "array", Items: &contract.Schema{Type: "integer"}}, schema)
	assert.True(t, schema.IsArray())
	assert.False(t, schema.IsObject())
	assert.False(t, schema.IsPrimitive())
	assert.False(t, schema.IsRef())
}

func TestSchemaFromDocument_ArrayComItemsVazio(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: array
items: {}
`))

	require.NotNil(t, schema.Items)
	assert.Equal(t, contract.Schema{}, *schema.Items)
	assert.True(t, schema.IsArray())
}

func TestSchemaFromDocument_ArraySemItems(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: array
`))

	assert.Nil(t, schema.Items)
	assert.True(t, schema.IsArray())
}

func TestSchemaFromDocument_ItemsQueNaoEhMapaEhIgnorado(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: array
items: string
`))

	assert.Equal(t, contract.Schema{Type: "array"}, schema)
}

func TestSchemaFromDocument_ItemsSemTypeEhArray(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `items:
  type: string
`))

	assert.Equal(t, contract.Schema{Items: &contract.Schema{Type: "string"}}, schema)
	assert.True(t, schema.IsArray())
	assert.False(t, schema.IsObject())
}

func TestSchemaFromDocument_Ref(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `ref: Pet
optional: true
`))

	assert.Equal(t, contract.Schema{Ref: "Pet", Optional: true}, schema)
	assert.True(t, schema.IsRef())
	assert.False(t, schema.IsObject())
	assert.False(t, schema.IsArray())
	assert.False(t, schema.IsPrimitive())
}

func TestSchemaFromDocument_RefComTypeOuEstruturaNaoEhRef(t *testing.T) {
	for name, source := range map[string]string{
		"type":       "ref: Pet\ntype: object\n",
		"properties": "ref: Pet\nproperties: {}\n",
		"items":      "ref: Pet\nitems: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			schema := contract.SchemaFromDocument(schemaDocument(t, source))

			assert.Equal(t, "Pet", schema.Ref)
			assert.False(t, schema.IsRef())
		})
	}
}

func TestSchemaFromDocument_Optional(t *testing.T) {
	assert.True(t, contract.SchemaFromDocument(contract.Document{"type": "string", "optional": true}).Optional)
	assert.False(t, contract.SchemaFromDocument(contract.Document{"type": "string", "optional": false}).Optional)
	assert.False(t, contract.SchemaFromDocument(contract.Document{"type": "string", "optional": "true"}).Optional)
	assert.False(t, contract.SchemaFromDocument(contract.Document{"type": "string"}).Optional)
}

func TestSchemaFromDocument_RecursaoAninhada(t *testing.T) {
	schema := contract.SchemaFromDocument(schemaDocument(t, `type: object
properties:
  users:
    type: array
    items:
      type: object
      properties:
        tags:
          type: array
          items:
            ref: Tag
            optional: true
`))

	assert.Equal(t, contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"users": {
				Type: "array",
				Items: &contract.Schema{
					Type: "object",
					Properties: map[string]contract.Schema{
						"tags": {
							Type:  "array",
							Items: &contract.Schema{Ref: "Tag", Optional: true},
						},
					},
				},
			},
		},
	}, schema)
}

func TestSchemaFromDocument_DocumentoNilOuVazioDaSchemaZero(t *testing.T) {
	assert.Equal(t, contract.Schema{}, contract.SchemaFromDocument(nil))
	assert.Equal(t, contract.Schema{}, contract.SchemaFromDocument(contract.Document{}))

	zero := contract.SchemaFromDocument(nil)
	assert.False(t, zero.IsObject())
	assert.False(t, zero.IsArray())
	assert.False(t, zero.IsPrimitive())
	assert.False(t, zero.IsRef())
}
