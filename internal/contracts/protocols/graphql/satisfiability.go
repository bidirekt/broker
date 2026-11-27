package graphql

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// reach is a type a query gets to, with the subgraphs that can be at it: the ones that resolved the field leading
// there, each with what the @provides on the path select from it below, and the type it declares the field returns.
type reach struct {
	typeName  string
	at        []subgraph
	provided  map[string]ast.SelectionSet
	own       map[string]string
	operation string
	path      []string
}

func (this reach) key() string {
	names := make([]string, 0, len(this.at))
	for _, member := range this.at {
		names = append(names, strings.TrimSpace(member.name+" "+this.own[member.name]+" "+formatSelections(this.provided[member.name])))
	}

	return this.typeName + " " + strings.Join(names, " ")
}

// below is the reach of a field the subgraphs at resolve: they provide below it what the path's @provides select
// under it, and what the field's own @provides selects.
func (this reach) below(field, coordinate, typeName string, at []subgraph) reach {
	provided := make(map[string]ast.SelectionSet, len(at))
	own := make(map[string]string, len(at))
	for _, member := range at {
		own[member.name] = member.namedTypes[coordinate]
		selected, _ := member.selectedUnder(this.provided[member.name], this.typeName, field)
		if selections := slices.Concat(selected, member.provides[coordinate]); len(selections) > 0 {
			provided[member.name] = selections
		}
	}

	return reach{typeName: typeName, at: at, provided: provided, own: own, operation: this.operation, path: append(slices.Clone(this.path), field)}
}

// checkSatisfiability walks every query the graph allows, from the root fields down, and reports once each field a
// query can't get to: the subgraphs that resolve it can't be entered, by a resolvable @key, from where the query is.
// A type reached from the same subgraphs is walked once, so recursion ends.
func (this *composition) checkSatisfiability() {
	var queue []reach
	for _, root := range this.roots() {
		for _, field := range this.fields[root.typeName] {
			coordinate := root.typeName + "." + field
			queue = append(queue, reach{typeName: root.typeName, operation: root.operation}.below(field, coordinate, this.namedTypes[coordinate], this.resolversOf(coordinate)))
		}
	}

	walked := map[string]bool{}
	unresolved := map[string]bool{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if walked[current.key()] {
			continue
		}

		walked[current.key()] = true

		switch this.kinds[current.typeName] {
		case graphqlmodel.ObjectKind:
			queue = append(queue, this.reachFields(current, unresolved)...)
		case graphqlmodel.InterfaceKind, graphqlmodel.UnionKind:
			queue = append(queue, this.reachMembers(current)...)
		}
	}
}

func (this *composition) reachFields(current reach, unresolved map[string]bool) []reach {
	entered := this.enteredFrom(current)

	var next []reach
	for _, field := range this.fields[current.typeName] {
		coordinate := current.typeName + "." + field
		resolvers := this.resolversOf(coordinate)
		reachable := slices.DeleteFunc(slices.Clone(resolvers), func(member subgraph) bool { return !containsSubgraph(entered, member) })

		for _, member := range current.at {
			if !containsSubgraph(reachable, member) && member.providesOnPath(current.provided[member.name], current.typeName, field) {
				reachable = append(reachable, member)
			}
		}

		if len(reachable) > 0 {
			next = append(next, current.below(field, coordinate, this.namedTypes[coordinate], reachable))

			continue
		}

		if !unresolved[coordinate] {
			unresolved[coordinate] = true
			this.addError(coordinate, fmt.Sprintf("%s %s.%s can't be resolved: no resolvable @key leads from %s to %s on %s",
				current.operation, strings.Join(current.path, "."), field, current.at[0].label, resolvers[0].label, current.typeName), current.at[0], resolvers[0])
		}
	}

	return next
}

// A subgraph returns, for a union or an interface, only the members it declares, or the member itself when its field
// returns that member, not the union or the interface the graph joins the field in.
func (this *composition) reachMembers(current reach) []reach {
	var next []reach
	for _, segment := range slices.Sorted(maps.Keys(this.schemas[current.typeName])) {
		if !model.IsMemberSegment(segment) {
			continue
		}

		at := slices.DeleteFunc(slices.Clone(current.at), func(member subgraph) bool {
			_, declares := member.schemas[current.typeName][segment]

			return !declares && current.own[member.name] != model.MemberKey(segment)
		})

		if len(at) > 0 {
			next = append(next, reach{typeName: model.MemberKey(segment), at: at, provided: current.provided, operation: current.operation, path: current.path})
		}
	}

	return next
}

// enteredFrom grows the subgraphs at a type by the jumps its resolvable @keys allow: a subgraph is entered when one
// already at the type resolves, or is provided on the path, every field of the key. A root type, reached by a field
// that returns it, enters every subgraph that defines it.
func (this *composition) enteredFrom(current reach) []subgraph {
	typeName := current.typeName
	entered := slices.Clone(current.at)
	root := slices.ContainsFunc(this.roots(), func(root rootType) bool { return root.typeName == typeName })

	for grown := true; grown; {
		grown = false

		for _, member := range this.subgraphs {
			if containsSubgraph(entered, member) {
				continue
			}

			if (root && member.definesType(typeName)) || slices.ContainsFunc(this.keysAt(member, typeName), func(key graphqlmodel.Directive) bool {
				selections, _ := parseFieldSet(key.Fields)

				return key.Directive == graphqlmodel.KeyDirective && key.Resolvable && slices.ContainsFunc(entered, func(from subgraph) bool {
					return from.resolvesAll(typeName, selections, current.provided[from.name])
				})
			}) {
				entered = append(entered, member)
				grown = true
			}
		}
	}

	return entered
}

// keysAt are the directives whose @keys enter a subgraph at a type: the type's own, or, at an implementation of an
// interface the subgraph marks @interfaceObject, the interface's.
func (this *composition) keysAt(member subgraph, typeName string) []graphqlmodel.Directive {
	if member.definesType(typeName) {
		return member.directives[typeName]
	}

	var keys []graphqlmodel.Directive
	for coordinate, directives := range member.directives {
		if _, implements := this.schemas[coordinate][model.MemberSegment(typeName)]; implements && member.marks(coordinate, graphqlmodel.InterfaceObjectDirective) {
			keys = append(keys, directives...)
		}
	}

	return keys
}

func (this subgraph) resolvesAll(typeName string, selections, provided ast.SelectionSet) bool {
	for _, selection := range selections {
		field := selection.(*ast.Field)
		coordinate := typeName + "." + field.Name
		selected, _ := this.selectedUnder(provided, typeName, field.Name)

		if !this.resolves(coordinate) && !this.providesOnPath(provided, typeName, field.Name) {
			return false
		}

		if len(field.SelectionSet) > 0 && !this.resolvesAll(this.namedTypes[coordinate], field.SelectionSet, slices.Concat(selected, this.provides[coordinate])) {
			return false
		}
	}

	return true
}

// A @provides counts only for a field the subgraph declares.
func (this subgraph) providesOnPath(provided ast.SelectionSet, typeName, field string) bool {
	_, selects := this.selectedUnder(provided, typeName, field)

	return selects && this.kinds[typeName+"."+field] == graphqlmodel.FieldKind
}

// selectedUnder tells whether a field set selects a field of a type, and what it selects below the field. An inline
// fragment selects at the type it names, and at every implementation or member of that type.
func (this subgraph) selectedUnder(selections ast.SelectionSet, typeName, field string) (ast.SelectionSet, bool) {
	var below ast.SelectionSet
	selects := false

	for _, selection := range selections {
		switch selection := selection.(type) {
		case *ast.Field:
			if selection.Name == field {
				below, selects = append(below, selection.SelectionSet...), true
			}
		case *ast.InlineFragment:
			_, isMember := this.schemas[selection.TypeCondition][model.MemberSegment(typeName)]
			if selection.TypeCondition == "" || selection.TypeCondition == typeName || isMember {
				fragmentBelow, fragmentSelects := this.selectedUnder(selection.SelectionSet, typeName, field)
				below, selects = append(below, fragmentBelow...), selects || fragmentSelects
			}
		}
	}

	return below, selects
}

func formatSelections(selections ast.SelectionSet) string {
	var written []string
	for _, selection := range selections {
		switch selection := selection.(type) {
		case *ast.Field:
			written = append(written, strings.TrimSpace(selection.Name+" "+formatSelections(selection.SelectionSet)))
		case *ast.InlineFragment:
			written = append(written, "... on "+selection.TypeCondition+" "+formatSelections(selection.SelectionSet))
		}
	}

	if len(written) == 0 {
		return ""
	}

	return "{ " + strings.Join(written, " ") + " }"
}

func (this *composition) resolversOf(coordinate string) []subgraph {
	var resolvers []subgraph
	for _, member := range this.subgraphs {
		if member.resolves(coordinate) {
			resolvers = append(resolvers, member)
		}
	}

	return append(resolvers, this.interfaceObjectResolvers[coordinate]...)
}

func containsSubgraph(subgraphs []subgraph, member subgraph) bool {
	return slices.ContainsFunc(subgraphs, func(other subgraph) bool { return other.name == member.name })
}
