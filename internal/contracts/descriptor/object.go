package descriptor

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

type Fields map[string]Node

type PresentFields map[string]any

func (this PresentFields) HasAny(names ...string) bool {
	for _, name := range names {
		if this[name] != nil {
			return true
		}
	}

	return false
}

type Rule func(present PresentFields) *violation.Violation

type ObjectNode struct {
	fields Fields
	rules  []Rule
}

func Object(fields Fields) ObjectNode {
	return ObjectNode{fields: fields}
}

func (this ObjectNode) Rules(rules ...Rule) ObjectNode {
	this.rules = rules

	return this
}

func (this ObjectNode) validate(value any, path string) []violation.Violation {
	mapping, ok := value.(map[string]any)

	if !ok && value != nil {
		return []violation.Violation{invalidKind(value, path, "mapping")}
	}

	violations := this.validateFields(mapping, path)

	return append(violations, this.applyRules(presentFields(mapping), path)...)
}

func (this ObjectNode) validateFields(mapping map[string]any, path string) []violation.Violation {
	var violations []violation.Violation

	for _, key := range slices.Sorted(maps.Keys(mapping)) {
		field, known := this.fields[key]
		if !known {
			violations = append(violations, violation.Violation{
				ErrorCode: "key.unknown",
				Path:      joinPath(path, key),
				Details:   map[string]string{"key": key},
			})

			continue
		}

		violations = append(violations, field.validate(mapping[key], joinPath(path, key))...)
	}

	return violations
}

func (this ObjectNode) applyRules(present PresentFields, path string) []violation.Violation {
	var violations []violation.Violation

	for _, rule := range this.rules {
		broken := rule(present)
		if broken == nil {
			continue
		}

		found := *broken
		found.Path = path
		violations = append(violations, found)
	}

	return violations
}

func presentFields(mapping map[string]any) PresentFields {
	present := PresentFields{}

	for key, value := range mapping {
		if value != nil {
			present[key] = value
		}
	}

	return present
}
