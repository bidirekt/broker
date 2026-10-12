package schemagraph

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

// Site is a field as written: the schema that declares it and its path relative to that schema.
type Site struct {
	Schema string
	Path   string
}

type Graph struct {
	catalog  model.Schemas
	segments map[string]map[string][]string
}

func New(catalog model.Schemas) *Graph {
	return &Graph{catalog: catalog, segments: map[string]map[string][]string{}}
}

// Root joins the bodies of one resource in their order. A `~` body declares nothing, so for senders it
// leaves the body itself optional.
func (this *Graph) Root(bodies []model.ResourceBody, join Join) Node {
	root := Node{graph: this, join: join}

	for _, body := range bodies {
		if !body.SchemaName.Valid {
			root.lacking = true

			continue
		}

		root.members = append(root.members, this.member(body.Source, Site{Schema: body.SchemaName.String}))
	}

	return root
}

// ChildPath appends a segment the way paths are spelled: "" + "name" → "name", "$" + "name" → "$.name",
// "items" + "[]" → "items[]", "pets" + "{}" → "pets{}".
func ChildPath(path, segment string) string {
	switch {
	case segment == "[]" || segment == "{}":
		return path + segment
	case path == "":
		return segment
	default:
		return path + "." + segment
	}
}

// member follows the refs of a site down to the field that gives it a shape, OR-ing optional and nullable
// along the chain. An undeclared schema or a schema met twice ends the chain on a field without a type.
func (this *Graph) member(source string, site Site) member {
	field, declared := this.catalog[site.Schema][site.Path]
	resolved := member{source: source, site: site, shape: site, fieldType: field.Type, optional: field.Optional, nullable: field.Nullable}
	visited := []Site{site}

	for declared && field.Ref != "" {
		target := Site{Schema: field.Ref}
		if slices.Contains(visited, target) {
			break
		}

		visited = append(visited, target)
		field, declared = this.catalog[target.Schema][target.Path]

		resolved.shape = target
		resolved.fieldType = field.Type
		resolved.optional = resolved.optional || field.Optional
		resolved.nullable = resolved.nullable || field.Nullable
	}

	return resolved
}

func (this *Graph) childSegments(shape Site) []string {
	bySchema, indexed := this.segments[shape.Schema]
	if !indexed {
		bySchema = segmentsByParent(this.catalog[shape.Schema])
		this.segments[shape.Schema] = bySchema
	}

	return bySchema[shape.Path]
}

func (this *Graph) declares(site Site) bool {
	_, declared := this.catalog[site.Schema][site.Path]

	return declared
}

func segmentsByParent(schema model.Schema) map[string][]string {
	byParent := map[string][]string{}

	for path := range schema {
		if parent, segment, nested := parentOf(path); nested {
			byParent[parent] = append(byParent[parent], segment)
		}
	}

	for _, segments := range byParent {
		slices.Sort(segments)
	}

	return byParent
}

func parentOf(path string) (parent string, segment string, nested bool) {
	switch {
	case path == "":
		return "", "", false
	case strings.HasSuffix(path, "[]"):
		return strings.TrimSuffix(path, "[]"), "[]", true
	case strings.HasSuffix(path, "{}"):
		return strings.TrimSuffix(path, "{}"), "{}", true
	}

	separator := strings.LastIndex(path, ".")
	if separator < 0 {
		return "", path, true
	}

	return path[:separator], path[separator+1:], true
}
