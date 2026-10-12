package model

import "strings"

// A ref field has no Type: Ref names the schema it points at, which is never copied into it.
type Field struct {
	Type     string
	Optional bool
	Nullable bool
	Ref      string
}

// Schema keys its fields by the path relative to the schema: "" (the schema itself), "city", "meta.x",
// "items[]", "items[].sku", "{}", "{}.name".
type Schema map[string]Field

const (
	ItemsSegment  = "[]"
	ValuesSegment = "{}"
)

func ChildPath(parent, segment string) string {
	switch {
	case segment == ItemsSegment || segment == ValuesSegment:
		return parent + segment
	case parent == "":
		return segment
	default:
		return parent + "." + segment
	}
}

func ParentOf(path string) (parent string, segment string, nested bool) {
	switch {
	case path == "":
		return "", "", false
	case strings.HasSuffix(path, ItemsSegment):
		return strings.TrimSuffix(path, ItemsSegment), ItemsSegment, true
	case strings.HasSuffix(path, ValuesSegment):
		return strings.TrimSuffix(path, ValuesSegment), ValuesSegment, true
	}

	separator := strings.LastIndex(path, ".")
	if separator < 0 {
		return "", path, true
	}

	return path[:separator], path[separator+1:], true
}

type Schemas map[string]Schema
