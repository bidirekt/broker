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
	fieldModelsFromSchema(fields, fieldPath(""), schema)
	schemas[name] = fields

	for _, field := range fields {
		if field.Ref != "" {
			schemaModelsReachableFrom(catalog, field.Ref, schemas)
		}
	}
}

func fieldModelsFromSchema(fields model.Schema, path fieldPath, schema contract.Schema) {
	switch {
	case schema.IsObject():
		fields[path.String()] = model.Field{Type: "object", Optional: schema.Optional, Nullable: schema.Nullable}

		for name, property := range schema.Properties {
			fieldModelsFromSchema(fields, path.Append(name), property)
		}

		if schema.Values != nil {
			fieldModelsFromSchema(fields, path.AppendMap(), *schema.Values)
		}

	case schema.IsArray():
		fields[path.String()] = model.Field{Type: "array", Optional: schema.Optional, Nullable: schema.Nullable}

		if schema.Items == nil {
			return
		}

		fieldModelsFromSchema(fields, path.AppendArray(), *schema.Items)

	case schema.IsPrimitive():
		fields[path.String()] = model.Field{Type: schema.Type, Optional: schema.Optional, Nullable: schema.Nullable}

	case schema.IsRef():
		fields[path.String()] = model.Field{Optional: schema.Optional, Nullable: schema.Nullable, Ref: schema.Ref}

	default:
		fields[path.String()] = model.Field{Type: schema.Type, Optional: schema.Optional, Nullable: schema.Nullable}
	}
}
