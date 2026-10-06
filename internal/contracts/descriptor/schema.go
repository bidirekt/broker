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
		"ref":         SchemaRef,
		"optional":    Flag,
	}).Rules(schemaHasShape, arrayRequiresItems)
}

func schemaHasShape(present PresentFields) *violation.Violation {
	if present.HasAny("type", "properties", "items", "ref") {
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
