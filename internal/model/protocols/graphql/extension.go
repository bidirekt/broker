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

const (
	KeyDirective          = "key"
	ShareableDirective    = "shareable"
	InaccessibleDirective = "inaccessible"
	ExternalDirective     = "external"
)

// Extension is what the SDL keeps that the schemas lose: whether a type is an input, the GraphQL name of a
// field's or argument's type (ID and String both become string), the federation directives that compose a
// graph, and the graph the subgraph belongs to. GraphSource and LinkSource name the files the graph and the
// federation @link came from; like every source, they stay out of the checksum and are never stored.
type Extension struct {
	Graph       string      `json:"graph,omitempty"`
	Directives  []Directive `json:"directives,omitempty"`
	Types       []TypeInfo  `json:"types,omitempty"`
	GraphSource string      `json:"-"`
	LinkSource  string      `json:"-"`
}

// TypeInfo is a type (Pet), a field (Pet.id) or an argument (Query.pet(id:)); NamedType is empty for a type.
type TypeInfo struct {
	Coordinate string `json:"coordinate"`
	Kind       string `json:"kind"`
	NamedType  string `json:"namedType,omitempty"`
}

// Directive is a federation directive that counts in the composition; Fields is the field set of a @key.
type Directive struct {
	Coordinate string `json:"coordinate"`
	Directive  string `json:"directive"`
	Fields     string `json:"fields,omitempty"`
	Resolvable bool   `json:"resolvable"`
}
