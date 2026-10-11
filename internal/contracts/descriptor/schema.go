package descriptor

import (
	"strings"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

func Schema() Node {
	return Object(Fields{
		"type":        SchemaType,
		"description": Text,
		"properties":  Map(PropertyName, Lazy(Schema)),
		"items":       Lazy(Schema),
		"values":      Lazy(Schema),
		"ref":         SchemaRef,
		"optional":    Flag,
		"nullable":    Flag,
	}).Rules(schemaHasShape, arrayRequiresItems, valuesExcludeProperties, valuesRequireObject)
}

func schemaHasShape(present PresentFields) *violation.Violation {
	if present.HasAny("type", "properties", "items", "values", "ref") {
		return nil
	}

	return &violation.Violation{
		ErrorCode: SchemaType.ErrorCode,
		Details: map[string]string{
			"value":   "",
			"allowed": strings.Join(SchemaType.Allowed, ", "),
		},
	}
}

func arrayRequiresItems(present PresentFields) *violation.Violation {
	if present["type"] != "array" || present["items"] != nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.array_without_items"}
}

func valuesExcludeProperties(present PresentFields) *violation.Violation {
	if present["values"] == nil || present["properties"] == nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.values_with_properties"}
}

func valuesRequireObject(present PresentFields) *violation.Violation {
	if present["values"] == nil || present["type"] == nil || present["type"] == "object" {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.values_without_object"}
}
