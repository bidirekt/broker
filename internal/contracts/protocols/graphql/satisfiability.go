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

// reach is a type a query gets to, with the subgraphs that can be at it: the ones that resolved the field leading there.
type reach struct {
	typeName  string
	at        []subgraph
	operation string
	path      []string
}

func (this reach) key() string {
	names := make([]string, 0, len(this.at))
	for _, member := range this.at {
		names = append(names, member.name)
	}

	return this.typeName + " " + strings.Join(names, " ")
}

// checkSatisfiability walks every query the graph allows, from the root fields down, and reports once each field a
// query can't get to: the subgraphs that resolve it can't be entered, by a resolvable @key, from where the query is.
// A type reached from the same subgraphs is walked once, so recursion ends.
func (this *composition) checkSatisfiability() {
	var queue []reach
	for _, root := range this.roots() {
		for _, field := range this.fields[root.typeName] {
			coordinate := root.typeName + "." + field
			queue = append(queue, reach{typeName: this.namedTypes[coordinate], at: this.resolversOf(coordinate), operation: root.operation, path: []string{field}})
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
	entered := this.enteredFrom(current.typeName, current.at)

	var next []reach
	for _, field := range this.fields[current.typeName] {
		coordinate := current.typeName + "." + field
		resolvers := this.resolversOf(coordinate)
		reachable := slices.DeleteFunc(slices.Clone(resolvers), func(member subgraph) bool { return !containsSubgraph(entered, member) })
		path := append(slices.Clone(current.path), field)

		if len(reachable) > 0 {
			next = append(next, reach{typeName: this.namedTypes[coordinate], at: reachable, operation: current.operation, path: path})

			continue
		}

		if !unresolved[coordinate] {
			unresolved[coordinate] = true
			this.addError(coordinate, fmt.Sprintf("%s %s can't be resolved: no resolvable @key leads from %s to %s on %s",
				current.operation, strings.Join(path, "."), current.at[0].label, resolvers[0].label, current.typeName), current.at[0], resolvers[0])
		}
	}

	return next
}

// A subgraph returns, for a union or an interface, only the members it declares.
func (this *composition) reachMembers(current reach) []reach {
	var next []reach
	for _, segment := range slices.Sorted(maps.Keys(this.schemas[current.typeName])) {
		if !model.IsMemberSegment(segment) {
			continue
		}

		at := slices.DeleteFunc(slices.Clone(current.at), func(member subgraph) bool {
			_, declares := member.schemas[current.typeName][segment]

			return !declares
		})

		if len(at) > 0 {
			next = append(next, reach{typeName: model.MemberKey(segment), at: at, operation: current.operation, path: current.path})
		}
	}

	return next
}

// enteredFrom grows the subgraphs at a type by the jumps its resolvable @keys allow: a subgraph is entered when one
// already at the type resolves every field of the key.
func (this *composition) enteredFrom(typeName string, at []subgraph) []subgraph {
	entered := slices.Clone(at)

	for grown := true; grown; {
		grown = false

		for _, member := range this.subgraphs {
			if containsSubgraph(entered, member) {
				continue
			}

			if slices.ContainsFunc(this.keysAt(member, typeName), func(key graphqlmodel.Directive) bool {
				selections, _ := parseFieldSet(key.Fields)

				return key.Directive == graphqlmodel.KeyDirective && key.Resolvable && slices.ContainsFunc(entered, func(from subgraph) bool {
					return from.resolvesAll(typeName, selections)
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

func (this subgraph) resolvesAll(typeName string, selections ast.SelectionSet) bool {
	for _, selection := range selections {
		field := selection.(*ast.Field)
		coordinate := typeName + "." + field.Name

		if !this.resolves(coordinate) || (len(field.SelectionSet) > 0 && !this.resolvesAll(this.namedTypes[coordinate], field.SelectionSet)) {
			return false
		}
	}

	return true
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
