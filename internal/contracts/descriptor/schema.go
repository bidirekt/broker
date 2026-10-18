package descriptor

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

func Property() Node {
	return schema()
}

func SchemaAt(position string) Node {
	return schema(optionalNotAllowedAt(position))
}

func schema(positionRules ...Rule) Node {
	return Object(Fields{
		"type":        SchemaType,
		"description": Text,
		"properties":  Map(PropertyName, Lazy(Property)),
		"items":       Lazy(func() Node { return SchemaAt("items") }),
		"values":      Lazy(func() Node { return SchemaAt("values") }),
		"ref":         SchemaRef,
		"optional":    Flag,
		"nullable":    Flag,
	}).Rules(append([]Rule{
		schemaRequiresTypeOrRef,
		refExcludes("type"),
		keyBelongsTo("properties", "object"),
		keyBelongsTo("items", "array"),
		keyBelongsTo("values", "map"),
		objectRequiresProperties,
		arrayRequiresItems,
		mapRequiresValues,
	}, positionRules...)...)
}

func schemaRequiresTypeOrRef(present PresentFields) *violation.Violation {
	if present["type"] != nil || (present["ref"] != nil && present["ref"] != "") {
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

func refExcludes(key string) Rule {
	return func(present PresentFields) *violation.Violation {
		if present["ref"] == nil || present[key] == nil {
			return nil
		}

		return keyNotAllowed(key, "ref")
	}
}

func keyBelongsTo(key string, schemaType string) Rule {
	return func(present PresentFields) *violation.Violation {
		if present[key] == nil {
			return nil
		}

		if present["ref"] != nil {
			return keyNotAllowed(key, "ref")
		}

		declaredType, valid := present["type"].(string)
		if !valid || declaredType == schemaType || !slices.Contains(SchemaType.Allowed, declaredType) {
			return nil
		}

		return keyNotAllowed(key, declaredType)
	}
}

func keyNotAllowed(key string, schemaType string) *violation.Violation {
	return &violation.Violation{
		ErrorCode: "schema.key_not_allowed",
		Details:   map[string]string{"key": key, "type": schemaType},
	}
}

func objectRequiresProperties(present PresentFields) *violation.Violation {
	if present["ref"] != nil || present["type"] != "object" {
		return nil
	}

	properties, isMapping := present["properties"].(map[string]any)
	if len(properties) > 0 || (present["properties"] != nil && !isMapping) {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.object_without_properties"}
}

func arrayRequiresItems(present PresentFields) *violation.Violation {
	if present["ref"] != nil || present["type"] != "array" || present["items"] != nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.array_without_items"}
}

func mapRequiresValues(present PresentFields) *violation.Violation {
	if present["ref"] != nil || present["type"] != "map" || present["values"] != nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.map_without_values"}
}

func optionalNotAllowedAt(position string) Rule {
	return func(present PresentFields) *violation.Violation {
		if present["optional"] == nil {
			return nil
		}

		return &violation.Violation{
			ErrorCode: "schema.optional_not_allowed",
			Details:   map[string]string{"position": position},
		}
	}
}
