package contract

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/model"
)

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

// SchemaChild is a schema nested in another: DocumentPath is where it is written below its parent
// ("properties;city", "items", "values"), Segment where its fields sit below the parent's.
type SchemaChild struct {
	DocumentPath string
	Segment      string
	Schema       Schema
}

func (this *Schema) Children() []SchemaChild {
	switch {
	case this.IsObject():
		children := make([]SchemaChild, 0, len(this.Properties))
		for _, name := range slices.Sorted(maps.Keys(this.Properties)) {
			children = append(children, SchemaChild{DocumentPath: "properties;" + name, Segment: name, Schema: this.Properties[name]})
		}

		return children

	case this.IsArray() && this.Items != nil:
		return []SchemaChild{{DocumentPath: "items", Segment: model.ItemsSegment, Schema: *this.Items}}

	case this.IsMap() && this.Values != nil:
		return []SchemaChild{{DocumentPath: "values", Segment: model.ValuesSegment, Schema: *this.Values}}

	default:
		return nil
	}
}

func (this *Schema) IsObject() bool {
	return this.Type == model.ObjectType
}

func (this *Schema) IsArray() bool {
	return this.Type == model.ArrayType
}

func (this *Schema) IsMap() bool {
	return this.Type == model.MapType
}

func (this *Schema) IsPrimitive() bool {
	return isPrimitiveType(this.Type)
}

func isPrimitiveType(schemaType string) bool {
	return schemaType == model.StringType ||
		schemaType == model.IntegerType ||
		schemaType == model.FloatType ||
		schemaType == model.NumberType ||
		schemaType == model.BooleanType
}

func (this *Schema) IsRef() bool {
	return this.Type == "" && this.Ref != ""
}
