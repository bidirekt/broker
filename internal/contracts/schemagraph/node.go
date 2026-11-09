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
	optional  bool
	nullable  bool
}

type Declaration struct {
	Source string
	Type   string
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
		declarations = append(declarations, Declaration{Source: declaration.source, Type: declaration.fieldType})
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

func (this Node) Child(segment string) Node {
	child := Node{graph: this.graph, join: this.join}

	for _, parent := range this.declarations {
		site := Site{Schema: parent.shape.Schema, Path: model.ChildPath(parent.shape.Path, segment)}
		if !this.graph.declares(site) {
			child.lacking = true

			continue
		}

		child.declarations = append(child.declarations, this.graph.declaration(parent.source, site))
	}

	return child
}

// SiteKey identifies the node by the fields it was reached through, which carry its optional and nullable.
func (this Node) SiteKey() string {
	var key strings.Builder
	if this.lacking {
		key.WriteString("lacking")
	}

	for _, declaration := range this.declarations {
		key.WriteString("\x00" + declaration.source + "\x00" + declaration.site.Schema + "\x00" + declaration.site.Path)
	}

	return key.String()
}

// ShapeKey identifies the node by the fields its refs lead to, which carry its type and children.
func (this Node) ShapeKey() string {
	var key strings.Builder
	for _, declaration := range this.declarations {
		key.WriteString("\x00" + declaration.source + "\x00" + declaration.shape.Schema + "\x00" + declaration.shape.Path)
	}

	return key.String()
}
