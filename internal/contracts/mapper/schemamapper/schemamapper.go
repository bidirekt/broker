package schemamapper

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/model"
)

// ToPropertyModels resolves a schema into its property models keyed by $-rooted path.
func ToPropertyModels(schemas contract.SchemasMap, root contract.Schema) map[string]model.Property {
	properties := map[string]model.Property{}

	propertyModelsFromSchema(schemas, properties, propertyPath("$"), root, contract.RefTrail{})

	return properties
}

func propertyModelsFromSchema(
	schemas contract.SchemasMap,
	properties map[string]model.Property,
	path propertyPath,
	schema contract.Schema,
	trail contract.RefTrail,
) {
	switch {
	case schema.IsObject():
		properties[path.String()] = model.NewProperty(path.String(), "object", schema.Optional, schema.Nullable)

		for name, property := range schema.Properties {
			propertyModelsFromSchema(schemas, properties, path.Append(name), property, trail)
		}

	case schema.IsArray():
		properties[path.String()] = model.NewProperty(path.String(), "array", schema.Optional, schema.Nullable)

		if schema.Items == nil {
			return
		}

		propertyModelsFromSchema(schemas, properties, path.AppendArray(), *schema.Items, trail)

	case schema.IsPrimitive():
		properties[path.String()] = model.NewProperty(path.String(), schema.Type, schema.Optional, schema.Nullable)

	case schema.IsRef():
		if trail.Contains(schema.Ref) {
			return
		}

		target := schemas[schema.Ref]
		target.Optional = target.Optional || schema.Optional
		target.Nullable = target.Nullable || schema.Nullable

		propertyModelsFromSchema(schemas, properties, path, target, trail.Enter(schema.Ref))

	default:
		properties[path.String()] = model.NewProperty(path.String(), schema.Type, schema.Optional, schema.Nullable)
	}
}
