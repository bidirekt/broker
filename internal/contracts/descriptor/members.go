package descriptor

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

// Members is the "members" of an enum or a union: entries checks what the type lists in it, and an empty
// list or map is rejected for both.
func Members(schemaType string, entries Node) Node {
	return membersNode{schemaType: schemaType, entries: entries}
}

type membersNode struct {
	schemaType string
	entries    Node
}

func (this membersNode) validate(value any, path string) []violation.Violation {
	if isEmptyCollection(value) {
		return []violation.Violation{{
			ErrorCode: "members.empty",
			Path:      path,
			Details:   map[string]string{"type": this.schemaType},
		}}
	}

	return this.entries.validate(value, path)
}

func isEmptyCollection(value any) bool {
	switch collection := value.(type) {
	case []any:
		return len(collection) == 0
	case map[string]any:
		return len(collection) == 0
	default:
		return false
	}
}

func EnumValues() Node {
	return enumValuesNode{}
}

type enumValuesNode struct{}

func (enumValuesNode) validate(value any, path string) []violation.Violation {
	violations := List(enumValueNode{}).validate(value, path)

	sequence, _ := value.([]any)
	seen := map[string]bool{}

	for index, item := range sequence {
		text, _ := item.(string)
		if text == "" {
			continue
		}

		if seen[text] {
			violations = append(violations, violation.Violation{
				ErrorCode: "members.duplicate",
				Path:      joinPath(path, strconv.Itoa(index)),
				Details:   map[string]string{"value": text},
			})
		}

		seen[text] = true
	}

	return violations
}

type enumValueNode struct{}

func (enumValueNode) validate(value any, path string) []violation.Violation {
	if text, isText := value.(string); isText && text != "" && !strings.ContainsRune(text, 0) {
		return nil
	}

	return []violation.Violation{{ErrorCode: "members.not_text", Path: path}}
}

func UnionMembers() Node {
	return unionMembersNode{}
}

type unionMembersNode struct{}

var unionMember = Object(Fields{"ref": SchemaRef})

func (unionMembersNode) validate(value any, path string) []violation.Violation {
	if value == nil {
		return nil
	}

	mapping, isMapping := value.(map[string]any)
	if !isMapping {
		return []violation.Violation{invalidKind(value, path, "mapping")}
	}

	var violations []violation.Violation

	for _, key := range slices.Sorted(maps.Keys(mapping)) {
		memberPath := joinPath(path, key)
		if rejected := MemberKey.reject(key, memberPath); rejected != nil {
			violations = append(violations, *rejected)

			continue
		}

		violations = append(violations, unionMember.validate(mapping[key], memberPath)...)

		if lacksRef(mapping[key]) {
			violations = append(violations, violation.Violation{
				ErrorCode: "member.ref_required",
				Path:      memberPath,
				Details:   map[string]string{"member": key},
			})
		}
	}

	return violations
}

// A member that is not a mapping is already reported as the wrong kind.
func lacksRef(member any) bool {
	if member == nil {
		return true
	}

	fields, isMapping := member.(map[string]any)
	ref, _ := fields["ref"].(string)

	return isMapping && ref == ""
}
