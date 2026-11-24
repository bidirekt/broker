package graphql

import (
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// subgraph is a member of a graph as the composition reads it: its schemas, the kind and named type of each
// coordinate, its federation directives and the coordinates its @key field sets name.
type subgraph struct {
	name       string
	label      string
	schemas    model.Schemas
	kinds      map[string]string
	namedTypes map[string]string
	directives map[string][]graphqlmodel.Directive
	keyFields  map[string]bool
}

func toSubgraph(contract *model.PersistedContract) subgraph {
	extension, _ := contract.Extensions[graphqlmodel.Protocol.String()].(graphqlmodel.Extension)

	member := subgraph{
		name:       contract.ParticipantName,
		label:      contract.ParticipantName + " " + contract.Version,
		schemas:    contract.Schemas[graphqlmodel.Namespace],
		kinds:      make(map[string]string, len(extension.Types)),
		namedTypes: make(map[string]string, len(extension.Types)),
		directives: map[string][]graphqlmodel.Directive{},
		keyFields:  map[string]bool{},
	}

	for _, info := range extension.Types {
		member.kinds[info.Coordinate] = info.Kind
		member.namedTypes[info.Coordinate] = info.NamedType
	}

	for _, directive := range extension.Directives {
		member.directives[directive.Coordinate] = append(member.directives[directive.Coordinate], directive)

		if directive.Directive == graphqlmodel.KeyDirective {
			member.addKeyFields(directive.Coordinate, toKeySelections(directive.Fields))
		}
	}

	return member
}

func (this subgraph) addKeyFields(typeName string, selections ast.SelectionSet) {
	for _, selection := range selections {
		field := selection.(*ast.Field)
		coordinate := typeName + "." + field.Name
		this.keyFields[coordinate] = true
		this.addKeyFields(this.namedTypes[coordinate], field.SelectionSet)
	}
}

// The field set was checked at publish, so it always parses into fields.
func toKeySelections(fieldSet string) ast.SelectionSet {
	document, err := parser.ParseQuery(&ast.Source{Input: "{" + fieldSet + "}"})
	if err != nil || len(document.Operations) != 1 {
		return nil
	}

	return document.Operations[0].SelectionSet
}

func (this subgraph) marks(coordinate, directive string) bool {
	return slices.ContainsFunc(this.directives[coordinate], func(kept graphqlmodel.Directive) bool {
		return kept.Directive == directive
	})
}

func (this subgraph) definesType(typeName string) bool {
	_, defined := this.schemas[typeName]

	return defined && this.kinds[typeName] != ""
}

func (this subgraph) resolves(coordinate string) bool {
	return this.kinds[coordinate] == graphqlmodel.FieldKind && !this.marks(coordinate, graphqlmodel.ExternalDirective)
}

// A field counts as @shareable when it, or its type, says so, or when a @key of the subgraph names it.
func (this subgraph) shares(typeName, coordinate string) bool {
	return this.marks(coordinate, graphqlmodel.ShareableDirective) ||
		this.marks(typeName, graphqlmodel.ShareableDirective) ||
		this.keyFields[coordinate]
}

// typed is the type of a field or an argument: its model fields from the outermost list down to the named type.
type typed struct {
	levels []model.Field
	named  string
}

// The levels of a property are the property and each of its list items: "reviews", "reviews[]", …
func (this subgraph) typedAt(schemaName, property, coordinate string) (typed, bool) {
	schema := this.schemas[schemaName]

	var levels []model.Field
	for path := property; ; path += model.ItemsSegment {
		level, declared := schema[path]
		if !declared {
			break
		}

		levels = append(levels, level)
	}

	return typed{levels: levels, named: this.namedTypes[coordinate]}, len(levels) > 0
}

func (this typed) sameShape(other typed) bool {
	return this.named == other.named && len(this.levels) == len(other.levels)
}

// String writes the type as GraphQL does: [Review!]!
func (this typed) String() string {
	var written strings.Builder
	for _, level := range this.levels {
		if level.Type == model.ArrayType {
			written.WriteString("[")
		}
	}

	written.WriteString(this.named)

	for index := len(this.levels) - 1; index >= 0; index-- {
		if !this.levels[index].Nullable {
			written.WriteString("!")
		}

		if index > 0 {
			written.WriteString("]")
		}
	}

	return written.String()
}

// propertiesOf names the fields of an object or input schema; their list items sit at "name[]".
func propertiesOf(schema model.Schema) []string {
	var properties []string
	for path := range schema {
		if path != "" && !strings.Contains(path, model.ItemsSegment) {
			properties = append(properties, path)
		}
	}

	slices.Sort(properties)

	return properties
}
