package descriptor

import (
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

func Property() Node {
	return CoreVocabulary().Property()
}

func SchemaAt(position string) Node {
	return CoreVocabulary().SchemaAt(position)
}

func (this Vocabulary) Property() Node {
	return this.grammar().property
}

func (this Vocabulary) SchemaAt(position string) Node {
	schema := this.grammar().schema

	return schema.Rules(append(slices.Clip(schema.rules), optionalNotAllowedAt(position))...)
}

type schemaGrammar struct {
	schema   ObjectNode
	property ObjectNode
}

func (this Vocabulary) grammar() *schemaGrammar {
	if this.built != nil {
		return this.built
	}

	bound := this
	bound.built = &schemaGrammar{}
	bound.built.schema = bound.schema(nil)
	bound.built.property = bound.schema(bound.PropertyKeys)

	return bound.built
}

// A key more than one type takes is chosen by the sibling "type"; a key of one type is validated under any type.
func (this Vocabulary) schema(extraKeys Keys) ObjectNode {
	schemaType := this.Type()
	fields := Fields{
		"type":        schemaType,
		"description": Description,
		"ref":         SchemaRef,
		"optional":    Flag,
		"nullable":    Flag,
	}
	rules := []Rule{schemaRequiresTypeOrRef(schemaType), refExcludes("type")}

	var keys []string
	grammars := map[string]ByType{}

	for _, typeVocabulary := range this.Types {
		for _, key := range slices.Sorted(maps.Keys(typeVocabulary.Keys)) {
			if grammars[key] == nil {
				keys = append(keys, key)
				grammars[key] = ByType{}
			}

			grammars[key][typeVocabulary.Name] = typeVocabulary.Keys[key](this)
		}
	}

	for _, key := range keys {
		owners := slices.Collect(maps.Keys(grammars[key]))

		fields[key] = grammars[key]
		if len(owners) == 1 {
			fields[key] = grammars[key][owners[0]]
		}

		rules = append(rules, keyBelongsTo(key, schemaType, owners...))
	}

	for _, typeVocabulary := range this.Types {
		rules = append(rules, typeVocabulary.Rules...)
	}

	for key, grammar := range extraKeys {
		fields[key] = grammar(this)
	}

	return Object(fields).Rules(rules...)
}

func schemaRequiresTypeOrRef(schemaType Enum) Rule {
	return func(present PresentFields) *violation.Violation {
		if present["type"] != nil || (present["ref"] != nil && present["ref"] != "") {
			return nil
		}

		return &violation.Violation{
			ErrorCode: schemaType.ErrorCode,
			Details: map[string]string{
				"value":   "",
				"allowed": strings.Join(schemaType.Allowed, ", "),
			},
		}
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

func keyBelongsTo(key string, schemaType Enum, owners ...string) Rule {
	return func(present PresentFields) *violation.Violation {
		if present[key] == nil {
			return nil
		}

		if present["ref"] != nil {
			return keyNotAllowed(key, "ref")
		}

		declaredType, valid := present["type"].(string)
		if !valid || slices.Contains(owners, declaredType) || !slices.Contains(schemaType.Allowed, declaredType) {
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
	if present["ref"] != nil || present["type"] != model.ObjectType {
		return nil
	}

	properties, isMapping := present["properties"].(map[string]any)
	if len(properties) > 0 || (present["properties"] != nil && !isMapping) {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.object_without_properties"}
}

func arrayRequiresItems(present PresentFields) *violation.Violation {
	if present["ref"] != nil || present["type"] != model.ArrayType || present["items"] != nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.array_without_items"}
}

func mapRequiresValues(present PresentFields) *violation.Violation {
	if present["ref"] != nil || present["type"] != model.MapType || present["values"] != nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.map_without_values"}
}

func membersRequired(schemaType string) Rule {
	return func(present PresentFields) *violation.Violation {
		if present["ref"] != nil || present["type"] != schemaType || present["members"] != nil {
			return nil
		}

		return &violation.Violation{
			ErrorCode: "members.required",
			Details:   map[string]string{"type": schemaType},
		}
	}
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
