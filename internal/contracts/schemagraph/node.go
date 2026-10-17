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
	graph   *Graph
	join    Join
	members []member
	lacking bool
}

type member struct {
	source    string
	site      Site
	shape     Site
	fieldType string
	optional  bool
	nullable  bool
}

type Member struct {
	Source string
	Type   string
}

func (this Node) Exists() bool {
	return len(this.members) > 0
}

// Type is the first body's: bodies that disagree are rejected at publish.
func (this Node) Type() string {
	if !this.Exists() {
		return ""
	}

	return this.members[0].fieldType
}

func (this Node) Optional() bool {
	if this.join == Senders {
		return this.lacking || slices.ContainsFunc(this.members, func(member member) bool { return member.optional })
	}

	return !slices.ContainsFunc(this.members, func(member member) bool { return !member.optional })
}

func (this Node) Nullable() bool {
	if this.join == Senders {
		return slices.ContainsFunc(this.members, func(member member) bool { return member.nullable })
	}

	return !slices.ContainsFunc(this.members, func(member member) bool { return !member.nullable })
}

func (this Node) Members() []Member {
	members := make([]Member, 0, len(this.members))
	for _, member := range this.members {
		members = append(members, Member{Source: member.source, Type: member.fieldType})
	}

	return members
}

// Field names the first body's field the node was reached at: "Address.city", "Order.items[].sku",
// "Pets{}.name", or the schema alone at its root.
func (this Node) Field() string {
	if !this.Exists() {
		return ""
	}

	site := this.members[0].site

	switch {
	case site.Path == "":
		return site.Schema
	case strings.HasPrefix(site.Path, model.ItemsSegment) || strings.HasPrefix(site.Path, model.ValuesSegment):
		return site.Schema + site.Path
	default:
		return site.Schema + "." + site.Path
	}
}

func (this Node) Segments() []string {
	var segments []string
	for _, member := range this.members {
		segments = append(segments, this.graph.childSegments(member.shape)...)
	}

	slices.Sort(segments)

	return slices.Compact(segments)
}

func (this Node) Child(segment string) Node {
	child := Node{graph: this.graph, join: this.join}

	for _, parent := range this.members {
		site := Site{Schema: parent.shape.Schema, Path: model.ChildPath(parent.shape.Path, segment)}
		if !this.graph.declares(site) {
			child.lacking = true

			continue
		}

		child.members = append(child.members, this.graph.member(parent.source, site))
	}

	return child
}

// SiteKey identifies the node by the fields it was reached through, which carry its optional and nullable.
func (this Node) SiteKey() string {
	var key strings.Builder
	if this.lacking {
		key.WriteString("lacking")
	}

	for _, member := range this.members {
		key.WriteString("\x00" + member.source + "\x00" + member.site.Schema + "\x00" + member.site.Path)
	}

	return key.String()
}

// ShapeKey identifies the node by the fields its refs lead to, which carry its type and children.
func (this Node) ShapeKey() string {
	var key strings.Builder
	for _, member := range this.members {
		key.WriteString("\x00" + member.source + "\x00" + member.shape.Schema + "\x00" + member.shape.Path)
	}

	return key.String()
}
