package graphql

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/lexer"
	"github.com/vektah/gqlparser/v2/parser"

	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

type keyUse struct {
	definition *ast.Definition
	directive  *ast.Directive
}

// The first @key in file order whose field set is not valid, or names a field its type lacks, is the error.
func (this federation) checkKeys(schema *ast.Schema, sources []*ast.Source) error {
	keyNames := this.localNamesOf("@" + graphqlmodel.KeyDirective)

	var uses []keyUse
	for _, name := range slices.Sorted(maps.Keys(schema.Types)) {
		definition := schema.Types[name]
		for _, directive := range definition.Directives {
			if slices.Contains(keyNames, directive.Name) {
				uses = append(uses, keyUse{definition: definition, directive: directive})
			}
		}
	}

	slices.SortFunc(uses, func(a, b keyUse) int {
		return cmp.Or(
			cmp.Compare(sourceIndex(sources, a.directive.Position.Src), sourceIndex(sources, b.directive.Position.Src)),
			cmp.Compare(a.directive.Position.Line, b.directive.Position.Line),
			cmp.Compare(a.directive.Position.Column, b.directive.Position.Column),
		)
	})

	for _, use := range uses {
		if err := checkKey(schema, use); err != nil {
			return err
		}
	}

	return nil
}

func sourceIndex(sources []*ast.Source, source *ast.Source) int {
	return slices.Index(sources, source)
}

func checkKey(schema *ast.Schema, use keyUse) error {
	fields := fieldsOf(use.directive)
	fieldSet := fields.Raw

	selections, valid := toFieldSet(fields)
	if !valid {
		return errorAtDirective(use.directive, "@%s(fields: %q) on %s is not a valid field set", use.directive.Name, fieldSet, use.definition.Name)
	}

	if name, owner, names := firstUndefinedField(schema, use.definition, selections); names {
		return errorAtDirective(use.directive, "@%s(fields: %q) on %s names %s, which %s doesn't define", use.directive.Name, fieldSet, use.definition.Name, name, owner)
	}

	return nil
}

// A @key whose definition, the user's own, lacks the fields argument has an empty field set.
func fieldsOf(directive *ast.Directive) *ast.Value {
	if fields := directive.Arguments.ForName("fields"); fields != nil {
		return fields.Value
	}

	return &ast.Value{Kind: ast.StringValue}
}

func toFieldSet(value *ast.Value) (ast.SelectionSet, bool) {
	if value.Kind != ast.StringValue && value.Kind != ast.BlockValue {
		return nil, false
	}

	return parseFieldSet(value.Raw)
}

// A field set selects fields only: a fragment or an inline fragment makes it invalid.
func parseFieldSet(fieldSet string) (ast.SelectionSet, bool) {
	selections, parsed := parseSelections(fieldSet)
	if !parsed || !onlyFields(selections) {
		return nil, false
	}

	return selections, true
}

// The field set of a @provides may also select through inline fragments.
func parseSelections(fieldSet string) (ast.SelectionSet, bool) {
	document, err := parser.ParseQuery(&ast.Source{Input: "{" + fieldSet + "}"})
	if err != nil || len(document.Operations) != 1 || len(document.Fragments) > 0 {
		return nil, false
	}

	return document.Operations[0].SelectionSet, true
}

func onlyFields(selections ast.SelectionSet) bool {
	for _, selection := range selections {
		field, isField := selection.(*ast.Field)
		if !isField {
			return false
		}

		if len(field.SelectionSet) > 0 && !onlyFields(field.SelectionSet) {
			return false
		}
	}

	return len(selections) > 0
}

// Commas are insignificant in GraphQL, so the field set is its tokens joined by single spaces; a punctuator token
// has no value, only its kind.
func toNormalizedFieldSet(fieldSet string) string {
	reader := lexer.New(&ast.Source{Input: fieldSet})

	var tokens []string
	for {
		token, err := reader.ReadToken()
		if err != nil || token.Kind == lexer.EOF {
			return strings.Join(tokens, " ")
		}

		tokens = append(tokens, cmp.Or(token.Value, token.Kind.String()))
	}
}

func firstUndefinedField(schema *ast.Schema, definition *ast.Definition, selections ast.SelectionSet) (name, owner string, found bool) {
	for _, selection := range selections {
		field := selection.(*ast.Field)

		declared := definition.Fields.ForName(field.Name)
		if declared == nil {
			return field.Name, definition.Name, true
		}

		if len(field.SelectionSet) == 0 {
			continue
		}

		if name, owner, found := firstUndefinedField(schema, schema.Types[declared.Type.Name()], field.SelectionSet); found {
			return name, owner, true
		}
	}

	return "", "", false
}
