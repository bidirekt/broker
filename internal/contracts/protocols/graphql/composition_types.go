package graphql

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func (this *composition) composeTypes() {
	for _, typeName := range this.typeNames() {
		definers := this.definersOf(typeName)
		if this.inaccessible(typeName) || !this.agreeOnKind(typeName, definers) {
			continue
		}

		kind := definers[0].kinds[typeName]
		this.kinds[typeName] = kind
		errorsBefore := len(this.errors)

		switch kind {
		case graphqlmodel.ObjectKind:
			this.composeObject(typeName, definers)
		case graphqlmodel.InputKind:
			this.schemas[typeName] = this.intersectInputs(typeName, func(field string) string { return typeName + "." + field }, definers)
		case graphqlmodel.InterfaceKind:
			this.composeInterface(typeName, definers)
		case graphqlmodel.UnionKind:
			this.schemas[typeName] = this.composeMembers(typeName, definers)
		case graphqlmodel.EnumKind:
			this.composeEnum(typeName, definers)
		default:
			this.schemas[typeName] = definers[0].schemas[typeName]
		}

		if len(this.errors) == errorsBefore {
			this.checkNotEmpty(typeName, kind, definers)
		}
	}
}

func (this *composition) agreeOnKind(typeName string, definers []subgraph) bool {
	first := definers[0]
	agree := true

	for _, other := range definers[1:] {
		if other.kinds[typeName] != first.kinds[typeName] {
			this.disagree(typeName, first, other, first.kinds[typeName], other.kinds[typeName])
			agree = false
		}
	}

	return agree
}

// disagree is the error of every conflict without a rule of its own: the element, then what each subgraph declares.
func (this *composition) disagree(coordinate string, one, other subgraph, oneDeclares, otherDeclares string) {
	this.addError(coordinate, fmt.Sprintf("%s and %s disagree on %s: %s in %s, %s in %s",
		one.label, other.label, coordinate, oneDeclares, one.name, otherDeclares, other.name), one, other)
}

// The fields of an object join; a field no subgraph resolves, only declares @external, is an error unless it is
// @inaccessible.
func (this *composition) composeObject(typeName string, definers []subgraph) {
	schema := model.Schema{"": {Type: model.ObjectType}}

	for _, field := range propertiesAcross(typeName, definers) {
		coordinate := typeName + "." + field
		resolvers := slices.DeleteFunc(slices.Clone(definers), func(member subgraph) bool { return !member.resolves(coordinate) })
		if this.inaccessible(coordinate) {
			continue
		}

		if len(resolvers) == 0 {
			this.requireResolver(coordinate, definers)

			continue
		}

		this.checkShared(coordinate, resolvers)
		this.composeField(schema, typeName, field, resolvers)
	}

	this.schemas[typeName] = schema
}

func (this *composition) requireResolver(coordinate string, definers []subgraph) {
	for _, member := range definers {
		if member.marks(coordinate, graphqlmodel.ExternalDirective) {
			this.addError(coordinate, fmt.Sprintf("%s marks %s @external, but no subgraph resolves it", member.label, coordinate), member)
		}
	}
}

// The fields of an interface join with no @shareable needed, and so do its implementations.
func (this *composition) composeInterface(typeName string, definers []subgraph) {
	schema := this.composeMembers(typeName, definers)

	for _, field := range propertiesAcross(typeName, definers) {
		coordinate := typeName + "." + field
		declarers := slices.DeleteFunc(slices.Clone(definers), func(member subgraph) bool { return member.kinds[coordinate] != graphqlmodel.FieldKind })
		if !this.inaccessible(coordinate) {
			this.composeField(schema, typeName, field, declarers)
		}
	}

	this.schemas[typeName] = schema
}

// An implementation of an interface takes each field of the interface it lacks that an @interfaceObject resolves:
// that subgraph resolves it for every implementation, as Apollo does.
func (this *composition) addInterfaceObjectFields() {
	for _, interfaceName := range slices.Sorted(maps.Keys(this.kinds)) {
		if this.kinds[interfaceName] != graphqlmodel.InterfaceKind {
			continue
		}

		for _, field := range this.fields[interfaceName] {
			resolvers := slices.DeleteFunc(slices.Clone(this.subgraphs), func(member subgraph) bool {
				return !member.marks(interfaceName, graphqlmodel.InterfaceObjectDirective) || !member.resolves(interfaceName+"."+field)
			})
			if len(resolvers) == 0 {
				continue
			}

			for _, implementation := range this.implementationsOf(interfaceName) {
				if _, declared := this.schemas[implementation][field]; !declared {
					this.addInterfaceObjectField(implementation, interfaceName, field, resolvers)
				}
			}
		}
	}
}

func (this *composition) addInterfaceObjectField(implementation, interfaceName, field string, resolvers []subgraph) {
	coordinate := implementation + "." + field

	for path := field; ; path += model.ItemsSegment {
		level, declared := this.schemas[interfaceName][path]
		if !declared {
			break
		}

		this.schemas[implementation][path] = level
	}

	this.fields[implementation] = append(this.fields[implementation], field)
	this.namedTypes[coordinate] = this.namedTypes[interfaceName+"."+field]
	this.interfaceObjectResolvers[coordinate] = resolvers
}

func (this *composition) implementationsOf(interfaceName string) []string {
	var implementations []string
	for _, segment := range slices.Sorted(maps.Keys(this.schemas[interfaceName])) {
		if model.IsMemberSegment(segment) && this.kinds[model.MemberKey(segment)] == graphqlmodel.ObjectKind {
			implementations = append(implementations, model.MemberKey(segment))
		}
	}

	return implementations
}

func (this *composition) composeField(schema model.Schema, typeName, field string, resolvers []subgraph) {
	coordinate := typeName + "." + field

	joined, joins := this.joinOutputs(typeName, field, coordinate, resolvers)
	if !joins {
		return
	}

	joined.levels[0].Arguments = this.composeArguments(typeName, field, resolvers)
	writeLevels(schema, field, joined.levels)
	this.fields[typeName] = append(this.fields[typeName], field)
	this.namedTypes[coordinate] = joined.named
}

// An object, an input or an enum left with no field or value, all @inaccessible or not shared, is no type GraphQL
// allows.
func (this *composition) checkNotEmpty(typeName, kind string, definers []subgraph) {
	schema := this.schemas[typeName]

	var element string
	switch {
	case (kind == graphqlmodel.ObjectKind || kind == graphqlmodel.InputKind) && len(propertiesOf(schema)) == 0:
		element = "field"
	case kind == graphqlmodel.EnumKind && len(schema[""].Members) == 0:
		element = "value"
	default:
		return
	}

	labels := make([]string, 0, len(definers))
	for _, member := range definers {
		labels = append(labels, member.label)
	}

	this.addError(typeName, fmt.Sprintf("%s has no %s left in the graph of %s", typeName, element, strings.Join(labels, " and ")), definers...)
}

func (this *composition) checkShared(coordinate string, resolvers []subgraph) {
	for index, one := range resolvers {
		for _, other := range resolvers[index+1:] {
			if !one.shares(coordinate) || !other.shares(coordinate) {
				this.addError(coordinate, fmt.Sprintf("%s and %s both resolve %s without @shareable%s", one.label, other.label, coordinate, this.strayOverrideOf(coordinate, one, other)), one, other)
			}
		}
	}
}

// Apollo ignores an @override whose from names no member, often a misspelled one, so the field stays shared.
func (this *composition) strayOverrideOf(coordinate string, pair ...subgraph) string {
	for _, member := range pair {
		if from, stray := member.strayOverrides[coordinate]; stray {
			names := make([]string, 0, len(this.subgraphs))
			for _, each := range this.subgraphs {
				names = append(names, each.name)
			}

			return fmt.Sprintf(": the @override(from: %q) of %s names no member of %s, whose members are %s", from, member.label, this.graph, strings.Join(names, " and "))
		}
	}

	return ""
}

// Outputs join when they have the same list depth and the same type, or when one is a member or an implementation of
// the other in any subgraph: the field takes the wider type, nullable where any subgraph returns null.
func (this *composition) joinOutputs(schemaName, property, coordinate string, resolvers []subgraph) (typed, bool) {
	first, _ := resolvers[0].typedAt(schemaName, property, coordinate)
	joined := typed{levels: slices.Clone(first.levels), named: first.named}
	joins := true

	for _, other := range resolvers[1:] {
		declared, _ := other.typedAt(schemaName, property, coordinate)
		if len(declared.levels) != len(joined.levels) || !this.related(joined.named, declared.named) {
			this.addError(coordinate, fmt.Sprintf("%s resolves %s as %s, but %s as %s", resolvers[0].label, coordinate, first, other.label, declared), resolvers[0], other)
			joins = false

			continue
		}

		wider := joined
		if this.isMemberOf(joined.named, declared.named) {
			wider = typed{levels: slices.Clone(declared.levels), named: declared.named}
		}

		for index := range wider.levels {
			wider.levels[index].Nullable = joined.levels[index].Nullable || declared.levels[index].Nullable
		}

		joined = wider
	}

	return joined, joins
}

func (this *composition) related(one, other string) bool {
	return one == other || this.isMemberOf(one, other) || this.isMemberOf(other, one)
}

// isMemberOf tells whether a subgraph declares the type a member of the union, or an implementation of the interface,
// an interface that implements it included.
func (this *composition) isMemberOf(typeName, supertype string) bool {
	return slices.ContainsFunc(this.subgraphs, func(member subgraph) bool {
		_, declared := member.schemas[supertype][model.MemberSegment(typeName)]

		return declared || slices.Contains(member.implements[typeName], supertype)
	})
}

// A field keeps its arguments schema only when an argument is left after the intersection.
func (this *composition) composeArguments(typeName, field string, definers []subgraph) string {
	schemaName := argumentsSchemaName(typeName, field)
	arguments := this.intersectInputs(schemaName, func(argument string) string { return typeName + "." + field + "(" + argument + ":)" }, definers)

	if len(arguments) == 1 {
		return ""
	}

	this.schemas[schemaName] = arguments

	return schemaName
}

// intersectInputs keeps the arguments or input fields every subgraph declares, non-null where any of them is; one a
// subgraph requires and another doesn't declare is an error.
func (this *composition) intersectInputs(schemaName string, coordinateOf func(string) string, definers []subgraph) model.Schema {
	schema := model.Schema{"": {Type: model.ObjectType}}

	for _, input := range propertiesAcross(schemaName, definers) {
		coordinate := coordinateOf(input)

		declared := make([]typed, len(definers))
		var missing []subgraph
		for index, member := range definers {
			var declares bool
			if declared[index], declares = member.typedAt(schemaName, input, coordinate); !declares {
				missing = append(missing, member)
			}
		}

		if this.inaccessible(coordinate) {
			this.requireAccessible(coordinate, definers, declared)

			continue
		}

		if len(missing) > 0 {
			this.requireDeclared(coordinate, definers, declared, missing)

			continue
		}

		if met, meets := this.meetInputs(coordinate, definers, declared); meets {
			writeLevels(schema, input, met.levels)
			this.namedTypes[coordinate] = met.named
		}
	}

	return schema
}

// An argument or input field the client must send can't be left out of the graph.
func (this *composition) requireAccessible(coordinate string, definers []subgraph, declared []typed) {
	for index, member := range definers {
		if len(declared[index].levels) > 0 && !declared[index].levels[0].Optional {
			this.addError(coordinate, fmt.Sprintf("%s requires %s, which is @inaccessible", member.label, coordinate), member)
		}
	}
}

func (this *composition) requireDeclared(coordinate string, definers []subgraph, declared []typed, missing []subgraph) {
	for index, member := range definers {
		if len(declared[index].levels) == 0 || declared[index].levels[0].Optional {
			continue
		}

		for _, lacking := range missing {
			this.addError(coordinate, fmt.Sprintf("%s requires %s, which %s doesn't declare", member.label, coordinate, lacking.label), member, lacking)
		}
	}
}

// Inputs of the same type meet: non-null and required where any subgraph says so.
func (this *composition) meetInputs(coordinate string, definers []subgraph, declared []typed) (typed, bool) {
	met := typed{levels: slices.Clone(declared[0].levels), named: declared[0].named}
	meets := true

	for index, other := range declared[1:] {
		if !other.sameShape(declared[0]) {
			this.disagree(coordinate, definers[0], definers[index+1], declared[0].String(), other.String())
			meets = false

			continue
		}

		for level := range met.levels {
			met.levels[level].Nullable = met.levels[level].Nullable && other.levels[level].Nullable
			met.levels[level].Optional = met.levels[level].Optional && other.levels[level].Optional
		}
	}

	return met, meets
}

// Every subgraph's members of a union or an interface join, with no @shareable needed.
func (this *composition) composeMembers(typeName string, definers []subgraph) model.Schema {
	schema := model.Schema{"": {Type: model.UnionType}}

	for _, member := range definers {
		for path, field := range member.schemas[typeName] {
			if model.IsMemberSegment(path) && !this.inaccessible(field.Ref) {
				schema[path] = field
			}
		}
	}

	return schema
}

// An output-only enum takes every subgraph's values, an input-only one the values all of them share, and one used both
// ways needs the same values everywhere.
func (this *composition) composeEnum(typeName string, definers []subgraph) {
	values := make([][]string, len(definers))
	for index, member := range definers {
		values[index] = slices.DeleteFunc(slices.Clone(member.schemas[typeName][""].Members), func(value string) bool {
			return this.inaccessible(typeName + "." + value)
		})
	}

	var composed []string
	switch use := this.enumUses[typeName]; {
	case use.input && use.output:
		for index, other := range definers[1:] {
			if !slices.Equal(values[0], values[index+1]) {
				this.addError(typeName, fmt.Sprintf("%s and %s declare different values for %s, used as input and output", definers[0].label, other.label, typeName), definers[0], other)
			}
		}

		composed = values[0]

	case use.input:
		composed = slices.DeleteFunc(slices.Clone(values[0]), func(value string) bool {
			return slices.ContainsFunc(values[1:], func(declared []string) bool { return !slices.Contains(declared, value) })
		})

	default:
		composed = slices.Compact(slices.Sorted(slices.Values(slices.Concat(values...))))
	}

	this.schemas[typeName] = model.Schema{"": {Type: model.EnumType, Members: composed}}
}

// A field, argument or input field left in the graph can't have a type that is out of it.
func (this *composition) checkReferences() {
	for _, coordinate := range slices.Sorted(maps.Keys(this.namedTypes)) {
		named := this.namedTypes[coordinate]
		if !this.inaccessible(named) {
			continue
		}

		for _, member := range this.subgraphs {
			if member.namedTypes[coordinate] == named {
				this.addError(coordinate, fmt.Sprintf("%s declares %s as %s, which is @inaccessible", member.label, coordinate, named), member)
			}
		}
	}
}

func propertiesAcross(schemaName string, definers []subgraph) []string {
	var properties []string
	for _, member := range definers {
		properties = append(properties, propertiesOf(member.schemas[schemaName])...)
	}

	slices.Sort(properties)

	return slices.Compact(properties)
}

func writeLevels(schema model.Schema, property string, levels []model.Field) {
	path := property
	for _, level := range levels {
		schema[path] = level
		path += model.ItemsSegment
	}
}
