package schemagraph

import (
	"slices"

	"github.com/bidirekt/broker/internal/model"
)

type Site struct {
	Namespace string
	Schema    string
	Path      string
}

type Graph struct {
	catalog   map[string]model.Schemas
	segments  map[Site]map[string][]string
	fieldName func(site Site) (name string, named bool)
}

func New(catalog map[string]model.Schemas) *Graph {
	return &Graph{catalog: catalog, segments: map[Site]map[string][]string{}}
}

// A site fieldName leaves unnamed keeps its schema's name.
func (this *Graph) NamingFields(fieldName func(site Site) (name string, named bool)) *Graph {
	this.fieldName = fieldName

	return this
}

// A `~` body declares nothing, so for senders it leaves the body itself optional.
func (this *Graph) Root(bodies []model.ResourceBody, join Join) Node {
	root := Node{graph: this, join: join}

	for _, body := range bodies {
		if !body.SchemaName.Valid {
			root.lacking = true

			continue
		}

		root.declarations = append(root.declarations, this.declaration(body.Source, Site{Namespace: body.Namespace, Schema: body.SchemaName.String}))
	}

	return root
}

// An undeclared schema or a schema met twice ends the chain on a field without a type.
func (this *Graph) declaration(source string, site Site) declaration {
	field, declared := this.field(site)
	resolved := declaration{
		source:    source,
		site:      site,
		shape:     site,
		fieldType: field.Type,
		members:   field.Members,
		scalar:    field.Scalar,
		arguments: field.Arguments,
		optional:  field.Optional,
		nullable:  field.Nullable,
	}
	visited := []Site{site}

	for declared && field.Ref != "" {
		target := Site{Namespace: site.Namespace, Schema: field.Ref}
		if slices.Contains(visited, target) {
			break
		}

		visited = append(visited, target)
		field, declared = this.field(target)

		resolved.shape = target
		resolved.fieldType = field.Type
		resolved.members = field.Members
		resolved.scalar = field.Scalar
		resolved.optional = resolved.optional || field.Optional
		resolved.nullable = resolved.nullable || field.Nullable
	}

	return resolved
}

func (this *Graph) childSegments(shape Site) []string {
	schema := Site{Namespace: shape.Namespace, Schema: shape.Schema}
	bySchema, indexed := this.segments[schema]
	if !indexed {
		bySchema = segmentsByParent(this.catalog[shape.Namespace][shape.Schema])
		this.segments[schema] = bySchema
	}

	return bySchema[shape.Path]
}

func (this *Graph) declares(site Site) bool {
	_, declared := this.field(site)

	return declared
}

func (this *Graph) field(site Site) (model.Field, bool) {
	field, declared := this.catalog[site.Namespace][site.Schema][site.Path]

	return field, declared
}

func segmentsByParent(schema model.Schema) map[string][]string {
	byParent := map[string][]string{}

	for path := range schema {
		if parent, segment, nested := model.ParentOf(path); nested {
			byParent[parent] = append(byParent[parent], segment)
		}
	}

	for _, segments := range byParent {
		slices.Sort(segments)
	}

	return byParent
}
