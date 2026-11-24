package graphql

import (
	"fmt"
	"slices"

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

		switch kind {
		case graphqlmodel.ObjectKind:
			this.composeObject(typeName, definers)
		case graphqlmodel.InputKind:
			this.schemas[typeName] = this.intersectInputs(typeName, func(field string) string { return typeName + "." + field }, definers)
		case graphqlmodel.InterfaceKind, graphqlmodel.UnionKind:
			this.composeMembers(typeName, definers)
		case graphqlmodel.EnumKind:
			this.composeEnum(typeName, definers)
		default:
			this.schemas[typeName] = definers[0].schemas[typeName]
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

// The fields of an object join; a field no subgraph resolves, only declares @external, is left out.
func (this *composition) composeObject(typeName string, definers []subgraph) {
	schema := model.Schema{"": {Type: model.ObjectType}}

	for _, field := range propertiesAcross(typeName, definers) {
		coordinate := typeName + "." + field
		resolvers := slices.DeleteFunc(slices.Clone(definers), func(member subgraph) bool { return !member.resolves(coordinate) })
		if this.inaccessible(coordinate) || len(resolvers) == 0 {
			continue
		}

		this.checkShared(typeName, coordinate, resolvers)

		joined, joins := this.joinOutputs(typeName, field, coordinate, resolvers)
		if !joins {
			continue
		}

		joined.levels[0].Arguments = this.composeArguments(typeName, field, resolvers)
		writeLevels(schema, field, joined.levels)
		this.fields[typeName] = append(this.fields[typeName], field)
		this.namedTypes[coordinate] = joined.named
	}

	this.schemas[typeName] = schema
}

func (this *composition) checkShared(typeName, coordinate string, resolvers []subgraph) {
	for index, one := range resolvers {
		for _, other := range resolvers[index+1:] {
			if !one.shares(typeName, coordinate) || !other.shares(typeName, coordinate) {
				this.addError(coordinate, fmt.Sprintf("%s and %s both resolve %s without @shareable", one.label, other.label, coordinate), one, other)
			}
		}
	}
}

// Outputs of the same type join, nullable where any subgraph returns null.
func (this *composition) joinOutputs(schemaName, property, coordinate string, resolvers []subgraph) (typed, bool) {
	first, _ := resolvers[0].typedAt(schemaName, property, coordinate)
	joined := typed{levels: slices.Clone(first.levels), named: first.named}
	joins := true

	for _, other := range resolvers[1:] {
		declared, _ := other.typedAt(schemaName, property, coordinate)
		if !declared.sameShape(first) {
			this.addError(coordinate, fmt.Sprintf("%s resolves %s as %s, but %s as %s", resolvers[0].label, coordinate, first, other.label, declared), resolvers[0], other)
			joins = false

			continue
		}

		for index := range joined.levels {
			joined.levels[index].Nullable = joined.levels[index].Nullable || declared.levels[index].Nullable
		}
	}

	return joined, joins
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
		if this.inaccessible(coordinate) {
			continue
		}

		declared := make([]typed, len(definers))
		var missing []subgraph
		for index, member := range definers {
			var declares bool
			if declared[index], declares = member.typedAt(schemaName, input, coordinate); !declares {
				missing = append(missing, member)
			}
		}

		if len(missing) > 0 {
			this.requireDeclared(coordinate, definers, declared, missing)

			continue
		}

		if met, meets := this.meetInputs(coordinate, definers, declared); meets {
			writeLevels(schema, input, met.levels)
		}
	}

	return schema
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
func (this *composition) composeMembers(typeName string, definers []subgraph) {
	schema := model.Schema{"": {Type: model.UnionType}}

	for _, member := range definers {
		for path, field := range member.schemas[typeName] {
			if path != "" && !this.inaccessible(field.Ref) {
				schema[path] = field
			}
		}
	}

	this.schemas[typeName] = schema
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
