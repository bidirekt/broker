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
	if this.Type != "" {
		return this.Type == "object"
	}

	return this.Properties != nil || this.Values != nil
}

func (this *Schema) IsArray() bool {
	if this.Type != "" {
		return this.Type == "array"
	}

	return this.Items != nil
}

func (this *Schema) IsPrimitive() bool {
	if this.Type != "" {
		return isPrimitiveType(this.Type)
	}

	return false
}

func isPrimitiveType(schemaType string) bool {
	return schemaType == "string" ||
		schemaType == "integer" ||
		schemaType == "float" ||
		schemaType == "number" ||
		schemaType == "boolean"
}

func (this *Schema) IsRef() bool {
	if this.Type != "" || this.Properties != nil || this.Items != nil || this.Values != nil {
		return false
	}

	return this.Ref != ""
}
