package model

// Field is one node of a schema as written. A ref field has no Type: Ref names the schema it points at,
// which is never copied into it.
type Field struct {
	Type     string
	Optional bool
	Nullable bool
	Ref      string
}

// Schema keys its fields by the path relative to the schema: "" (the schema itself), "city", "meta.x",
// "items[]", "items[].sku", "{}", "{}.name".
type Schema map[string]Field

type Schemas map[string]Schema // schema name → its fields
