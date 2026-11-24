package schemagraph

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

type Join int

const (
	Senders Join = iota
	Readers
)

type Node struct {
	graph        *Graph
	join         Join
	declarations []declaration
	lacking      bool
}

type declaration struct {
	source    string
	site      Site
	shape     Site
	fieldType string
	members   []string
	scalar    string
	arguments string
	optional  bool
	nullable  bool
}

type Declaration struct {
	Source    string
	Type      string
	Members   []string
	Scalar    string
	Arguments string
}

func (this Node) Exists() bool {
	return len(this.declarations) > 0
}

// Type is the first body's: bodies that disagree are rejected at publish.
func (this Node) Type() string {
	if !this.Exists() {
		return ""
	}

	return this.declarations[0].fieldType
}

// Members are the first body's enum values, sorted: bodies that disagree are rejected at publish.
func (this Node) Members() []string {
	if !this.Exists() {
		return nil
	}

	return this.declarations[0].members
}

// Scalar is the first body's: bodies that disagree are rejected at publish.
func (this Node) Scalar() string {
	if !this.Exists() {
		return ""
	}

	return this.declarations[0].scalar
}

// Arguments is the first body's: bodies that disagree are rejected at publish.
func (this Node) Arguments() string {
	if !this.Exists() {
		return ""
	}

	return this.declarations[0].arguments
}

// A field without arguments takes an empty object, never nothing: an argument it newly requires must break.
func (this Node) ArgumentsRoot(join Join) Node {
	root := Node{graph: this.graph, join: join}

	for _, declaration := range this.declarations {
		if declaration.arguments != "" {
			root.declarations = append(root.declarations, this.graph.declaration(declaration.source, Site{Namespace: declaration.site.Namespace, Schema: declaration.arguments}))
		}
	}

	if !root.Exists() {
		root.declarations = []declaration{{fieldType: model.ObjectType}}
	}

	return root
}

func (this Node) Optional() bool {
	if this.join == Senders {
		return this.lacking || slices.ContainsFunc(this.declarations, func(declaration declaration) bool { return declaration.optional })
	}

	return !slices.ContainsFunc(this.declarations, func(declaration declaration) bool { return !declaration.optional })
}

func (this Node) Nullable() bool {
	if this.join == Senders {
		return slices.ContainsFunc(this.declarations, func(declaration declaration) bool { return declaration.nullable })
	}

	return !slices.ContainsFunc(this.declarations, func(declaration declaration) bool { return !declaration.nullable })
}

func (this Node) Declarations() []Declaration {
	declarations := make([]Declaration, 0, len(this.declarations))
	for _, declaration := range this.declarations {
		declarations = append(declarations, Declaration{
			Source:    declaration.source,
			Type:      declaration.fieldType,
			Members:   declaration.members,
			Scalar:    declaration.scalar,
			Arguments: declaration.arguments,
		})
	}

	return declarations
}

// Field names the first body's field the node was reached at: "Address.city", "Order.items[].sku",
// "Pets{}.name", or the schema alone at its root.
func (this Node) Field() string {
	if !this.Exists() {
		return ""
	}

	site := this.declarations[0].site

	return model.FieldName(site.Schema, site.Path)
}

func (this Node) Segments() []string {
	var segments []string
	for _, declaration := range this.declarations {
		segments = append(segments, this.graph.childSegments(declaration.shape)...)
	}

	slices.Sort(segments)

	return slices.Compact(segments)
}

// A union member some bodies leave out is an alternative they never send, not a field they may omit.
func (this Node) Child(segment string) Node {
	child := Node{graph: this.graph, join: this.join}

	for _, parent := range this.declarations {
		site := Site{Namespace: parent.shape.Namespace, Schema: parent.shape.Schema, Path: model.ChildPath(parent.shape.Path, segment)}
		if !this.graph.declares(site) {
			child.lacking = !model.IsMemberSegment(segment)

			continue
		}

		child.declarations = append(child.declarations, this.graph.declaration(parent.source, site))
	}

	return child
}

func (this Node) Declaring(segment string) Node {
	declaring := Node{graph: this.graph, join: this.join}
	for _, declaration := range this.declarations {
		if this.graph.declares(Site{Namespace: declaration.shape.Namespace, Schema: declaration.shape.Schema, Path: model.ChildPath(declaration.shape.Path, segment)}) {
			declaring.declarations = append(declaring.declarations, declaration)
		}
	}

	return declaring
}

// SiteKey identifies the node by the fields it was reached through, which carry its optional and nullable.
func (this Node) SiteKey() string {
	var key strings.Builder
	if this.lacking {
		key.WriteString("lacking")
	}

	for _, declaration := range this.declarations {
		key.WriteString("\x00" + declaration.source + "\x00" + declaration.site.Namespace + "\x00" + declaration.site.Schema + "\x00" + declaration.site.Path)
	}

	return key.String()
}

// ShapeKey identifies the node by the fields its refs lead to, which carry its type and children.
func (this Node) ShapeKey() string {
	var key strings.Builder
	for _, declaration := range this.declarations {
		key.WriteString("\x00" + declaration.source + "\x00" + declaration.shape.Namespace + "\x00" + declaration.shape.Schema + "\x00" + declaration.shape.Path)
	}

	return key.String()
}
