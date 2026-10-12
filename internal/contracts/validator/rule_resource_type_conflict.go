package validator

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

// Only consumed resources merge, so only they can disagree with themselves.
func conflictingPropertyTypes(declarations fragmentmapper.Declarations) []violation.Violation {
	graph := schemagraph.New(fragmentmapper.ToSchemaModels(declarations))

	var violations []violation.Violation

	for _, declaration := range fragmentmapper.ToMergedResourceDeclarations(declarations) {
		if !declaration.Resource.IsConsumer() {
			continue
		}

		bodies := describedBodies(declaration.Resource.Bodies, declarations.Catalog)
		if len(bodies) == 0 {
			continue
		}

		conflicts := &typeConflicts{
			path:     declaration.Path.String(),
			resource: declaration.Resource.Describe(),
			compared: map[string]bool{},
		}
		conflicts.compare(graph.Root(bodies, schemagraph.Readers), "$")

		violations = append(violations, conflicts.violations...)
	}

	return violations
}

func describedBodies(bodies []model.ResourceBody, catalog contract.SchemasMap) []model.ResourceBody {
	var described []model.ResourceBody

	for _, body := range bodies {
		if _, declared := catalog[body.SchemaName.String]; body.SchemaName.Valid && declared {
			described = append(described, body)
		}
	}

	return described
}

// typeConflicts compares each joined shape once, so a conflict inside a schema used in many places is reported once.
type typeConflicts struct {
	path       string
	resource   string
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
			Path:      this.path,
			Source:    member.Source,
			Details: map[string]string{
				"resource":     this.resource,
				"property":     property,
				"type":         member.Type,
				"declaredIn":   members[0].Source,
				"declaredType": members[0].Type,
			},
		})
	}

	for _, segment := range node.Segments() {
		this.compare(node.Child(segment), model.ChildPath(property, segment))
	}
}
