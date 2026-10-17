package contract

type SchemasMap map[string]Schema

type Schema struct {
	Type        string
	Description string
	Properties  map[string]Schema
	Items       *Schema
	Values      *Schema
	Ref         string
	Optional    bool
	Nullable    bool
}

func SchemaFromDocument(document Document) Schema {
	schema := Schema{
		Type:        document.Text("type"),
		Description: document.Text("description"),
		Ref:         document.Text("ref"),
		Optional:    document.Flag("optional"),
		Nullable:    document.Flag("nullable"),
	}

	if properties, written := document["properties"].(map[string]any); written {
		schema.Properties = make(map[string]Schema, len(properties))

		for name := range properties {
			schema.Properties[name] = SchemaFromDocument(Document(properties).Mapping(name))
		}
	}

	if items, written := document["items"].(map[string]any); written {
		itemsSchema := SchemaFromDocument(items)
		schema.Items = &itemsSchema
	}

	if values, written := document["values"].(map[string]any); written {
		valuesSchema := SchemaFromDocument(values)
		schema.Values = &valuesSchema
	}

	return schema
}

func (this *Schema) IsObject() bool {
	return this.Type == "object"
}

func (this *Schema) IsArray() bool {
	return this.Type == "array"
}

func (this *Schema) IsMap() bool {
	return this.Type == "map"
}

func (this *Schema) IsPrimitive() bool {
	return isPrimitiveType(this.Type)
}

func isPrimitiveType(schemaType string) bool {
	return schemaType == "string" ||
		schemaType == "integer" ||
		schemaType == "float" ||
		schemaType == "number" ||
		schemaType == "boolean"
}

func (this *Schema) IsRef() bool {
	return this.Type == "" && this.Ref != ""
}
