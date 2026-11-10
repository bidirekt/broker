package model

import (
	"slices"
	"strings"
)

// A ref field has no Type: Ref names the schema it points at, which is never copied into it.
// Members holds an enum's values, sorted; a union's members are its child fields at MemberSegment(key), one ref each.
type Field struct {
	Type     string
	Optional bool
	Nullable bool
	Ref      string
	Members  []string
}

func (this Field) Equal(other Field) bool {
	return this.Type == other.Type &&
		this.Optional == other.Optional &&
		this.Nullable == other.Nullable &&
		this.Ref == other.Ref &&
		slices.Equal(this.Members, other.Members)
}

// Schema keys its fields by the path relative to the schema: "" (the schema itself), "city", "meta.x",
// "items[]", "items[].sku", "{}", "{}.name", "payment(card)".
type Schema map[string]Field

const (
	ObjectType  = "object"
	ArrayType   = "array"
	MapType     = "map"
	StringType  = "string"
	IntegerType = "integer"
	FloatType   = "float"
	NumberType  = "number"
	BooleanType = "boolean"
	EnumType    = "enum"
	UnionType   = "union"
)

const (
	ItemsSegment  = "[]"
	ValuesSegment = "{}"
)

func MemberSegment(key string) string {
	return "(" + key + ")"
}

func MemberKey(segment string) string {
	return strings.TrimSuffix(strings.TrimPrefix(segment, "("), ")")
}

func IsMemberSegment(segment string) bool {
	return strings.HasPrefix(segment, "(")
}

// The segments the broker writes ("[]", "{}", "(card)") join without a dot: no name may contain them.
func startsWithoutDot(path string) bool {
	return strings.HasPrefix(path, ItemsSegment) || strings.HasPrefix(path, ValuesSegment) || IsMemberSegment(path)
}

func ChildPath(parent, segment string) string {
	switch {
	case startsWithoutDot(segment):
		return parent + segment
	case parent == "":
		return segment
	default:
		return parent + "." + segment
	}
}

func FieldName(schema, path string) string {
	if path == "" || startsWithoutDot(path) {
		return schema + path
	}

	return schema + "." + path
}

func ParentOf(path string) (parent string, segment string, nested bool) {
	switch {
	case path == "":
		return "", "", false
	case strings.HasSuffix(path, ItemsSegment):
		return strings.TrimSuffix(path, ItemsSegment), ItemsSegment, true
	case strings.HasSuffix(path, ValuesSegment):
		return strings.TrimSuffix(path, ValuesSegment), ValuesSegment, true
	case strings.HasSuffix(path, ")"):
		opening := strings.LastIndex(path, "(")

		return path[:opening], path[opening:], true
	}

	separator := strings.LastIndex(path, ".")
	if separator < 0 {
		return "", path, true
	}

	return path[:separator], path[separator+1:], true
}

type Schemas map[string]Schema
