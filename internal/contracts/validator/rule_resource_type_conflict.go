package validator

import (
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

type consumedResource struct {
	path     string
	resource string
	bodies   []model.ResourceBody
}

// Only consumed resources merge, so only they can disagree with themselves.
func conflictingPropertyTypes(declarations fragmentmapper.Declarations) []violation.Violation {
	var resources []*consumedResource

	byPath := map[string]*consumedResource{}

	for _, declaration := range declarations.Resources {
		if !declaration.Resource.IsConsumer() {
			continue
		}

		if _, declared := declarations.Catalog[declaration.SchemaName]; declaration.WithoutSchema || !declared {
			continue
		}

		path := declaration.Path.String()

		resource, tracked := byPath[path]
		if !tracked {
			resource = &consumedResource{path: path, resource: declaration.Resource.Describe()}
			byPath[path] = resource
			resources = append(resources, resource)
		}

		resource.bodies = append(resource.bodies, declaration.Resource.Bodies...)
	}

	graph := schemagraph.New(fragmentmapper.ToSchemaModels(declarations))

	var violations []violation.Violation

	for _, resource := range resources {
		conflicts := &typeConflicts{resource: resource, compared: map[string]bool{}}
		conflicts.compare(graph.Root(resource.bodies, schemagraph.Readers), "$")

		violations = append(violations, conflicts.violations...)
	}

	return violations
}

// typeConflicts compares each joined shape once, so a conflict inside a schema used in many places is reported once.
type typeConflicts struct {
	resource   *consumedResource
	compared   map[string]bool
	violations []violation.Violation
}

func (this *typeConflicts) compare(node schemagraph.Node, property string) {
	shapes := node.ShapeKey()
	if this.compared[shapes] {
		return
	}

	this.compared[shapes] = true

	members := node.Members()
	for _, member := range members[1:] {
		if member.Type == members[0].Type {
			continue
		}

		this.violations = append(this.violations, violation.Violation{
			ErrorCode: "resource.type_conflict",
			Path:      this.resource.path,
			Source:    member.Source,
			Details: map[string]string{
				"resource":     this.resource.resource,
				"property":     property,
				"type":         member.Type,
				"declaredIn":   members[0].Source,
				"declaredType": members[0].Type,
			},
		})
	}

	for _, segment := range node.Segments() {
		this.compare(node.Child(segment), schemagraph.ChildPath(property, segment))
	}
}
