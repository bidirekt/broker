package validator

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

// Only consumed resources merge, so only they can disagree with themselves.
func conflictingPropertyTypes(declarations protocol.Declarations) []violation.Violation {
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
			path:      declaration.Path.String(),
			resource:  declaration.Resource.Describe(),
			compared:  map[[2]string]bool{},
			descended: map[string]bool{},
		}
		conflicts.compare(graph.Root(bodies, schemagraph.Readers), "$")

		violations = append(violations, conflicts.violations...)
	}

	return violations
}

func describedBodies(bodies []model.ResourceBody, catalog map[string]contract.SchemasMap) []model.ResourceBody {
	var described []model.ResourceBody

	for _, body := range bodies {
		if _, declared := catalog[body.Namespace][body.SchemaName.String]; body.SchemaName.Valid && declared {
			described = append(described, body)
		}
	}

	return described
}

// The arguments belong to the fields a node is reached through, and the children to the shapes their refs lead to, so
// each joined shape is compared once per arguments of those fields and descended into once: a conflict inside a schema
// used in many places is reported once.
type typeConflicts struct {
	path       string
	resource   string
	compared   map[[2]string]bool
	descended  map[string]bool
	violations []violation.Violation
}

func (this *typeConflicts) compare(node schemagraph.Node, property string) {
	declarations := node.Declarations()
	shapes := node.ShapeKey()

	fields := [2]string{shapes, argumentsKey(declarations)}
	if this.compared[fields] {
		return
	}

	this.compared[fields] = true

	for _, declaration := range declarations[1:] {
		this.compareDeclarations(declarations[0], declaration, property)
	}

	if this.descended[shapes] {
		return
	}

	this.descended[shapes] = true

	for _, segment := range node.Segments() {
		this.compare(node.Child(segment), model.ChildPath(property, segment))
	}
}

func argumentsKey(declarations []schemagraph.Declaration) string {
	var key strings.Builder
	for _, declaration := range declarations {
		key.WriteString("\x00" + declaration.Arguments)
	}

	return key.String()
}

func (this *typeConflicts) compareDeclarations(first, other schemagraph.Declaration, property string) {
	sameType := other.Type == first.Type && other.Scalar == first.Scalar
	sameMembers := slices.Equal(other.Members, first.Members)
	if sameType && sameMembers && other.Arguments == first.Arguments {
		return
	}

	details := map[string]string{
		"resource":     this.resource,
		"property":     property,
		"type":         typeName(other),
		"declaredIn":   first.Source,
		"declaredType": typeName(first),
	}

	if sameType && !sameMembers {
		details["values"] = strings.Join(other.Members, ", ")
		details["declaredValues"] = strings.Join(first.Members, ", ")
	}

	if sameType && sameMembers {
		details["arguments"] = other.Arguments
		details["declaredArguments"] = first.Arguments
	}

	this.violations = append(this.violations, violation.Violation{
		ErrorCode: "resource.type_conflict",
		Path:      this.path,
		Source:    other.Source,
		Details:   details,
	})
}

func typeName(declaration schemagraph.Declaration) string {
	if declaration.Type == model.ScalarType {
		return declaration.Scalar
	}

	return declaration.Type
}
