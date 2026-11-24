package graphql

const (
	ObjectKind    = "object"
	InputKind     = "input"
	InterfaceKind = "interface"
	UnionKind     = "union"
	EnumKind      = "enum"
	ScalarKind    = "scalar"
	FieldKind     = "field"
	ArgumentKind  = "argument"
)

// Extension is what the SDL keeps that the schemas lose: whether a type is an input, and the GraphQL name of a
// field's or argument's type (ID and String both become string).
type Extension struct {
	Types []TypeInfo `json:"types,omitempty"`
}

// TypeInfo is a type (Pet), a field (Pet.id) or an argument (Query.pet(id:)); NamedType is empty for a type.
type TypeInfo struct {
	Coordinate string `json:"coordinate"`
	Kind       string `json:"kind"`
	NamedType  string `json:"namedType,omitempty"`
}
