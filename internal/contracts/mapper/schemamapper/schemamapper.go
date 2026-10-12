package schemamapper

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/model"
)

func ToSchemaModels(catalog contract.SchemasMap, roots []string) model.Schemas {
	schemas := model.Schemas{}

	for _, root := range roots {
		schemaModelsReachableFrom(catalog, root, schemas)
	}

	return schemas
}

// An undeclared name is the validator's to report, so it is skipped here.
func schemaModelsReachableFrom(catalog contract.SchemasMap, name string, schemas model.Schemas) {
	schema, declared := catalog[name]
	if _, converted := schemas[name]; converted || !declared {
		return
	}

	fields := model.Schema{}
	fieldModelsFromSchema(fields, "", schema)
	schemas[name] = fields

	for _, field := range fields {
		if field.Ref != "" {
			schemaModelsReachableFrom(catalog, field.Ref, schemas)
		}
	}
}

func fieldModelsFromSchema(fields model.Schema, path string, schema contract.Schema) {
	switch {
	case schema.IsObject():
		fields[path] = model.Field{Type: "object", Optional: schema.Optional, Nullable: schema.Nullable}

		for name, property := range schema.Properties {
			fieldModelsFromSchema(fields, model.ChildPath(path, name), property)
		}

		if schema.Values != nil {
			fieldModelsFromSchema(fields, model.ChildPath(path, model.ValuesSegment), *schema.Values)
		}

	case schema.IsArray():
		fields[path] = model.Field{Type: "array", Optional: schema.Optional, Nullable: schema.Nullable}

		if schema.Items == nil {
			return
		}

		fieldModelsFromSchema(fields, model.ChildPath(path, model.ItemsSegment), *schema.Items)

	case schema.IsPrimitive():
		fields[path] = model.Field{Type: schema.Type, Optional: schema.Optional, Nullable: schema.Nullable}

	case schema.IsRef():
		fields[path] = model.Field{Optional: schema.Optional, Nullable: schema.Nullable, Ref: schema.Ref}

	default:
		fields[path] = model.Field{Type: schema.Type, Optional: schema.Optional, Nullable: schema.Nullable}
	}
}
