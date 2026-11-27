package graphql

import (
	"bytes"
	_ "embed"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/parser"

	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

const federationURLPrefix = "https://specs.apollo.dev/federation/"

const federationNamespace = "federation"

const linkDefinitions = `directive @link(url: String!, as: String, for: link__Purpose, import: [link__Import]) repeatable on SCHEMA
scalar link__Import
enum link__Purpose { SECURITY EXECUTION }
`

// The definitions of the newest federation 2 version we know serve every federation/v2.x link: no version since 2.0
// changed what an SDL means to the comparison.
//
//go:embed federation.graphql
var federationDefinitions string

//go:embed federation1.graphql
var federation1Definitions string

var federation2Version = regexp.MustCompile(`^v2\.\d+$`)

var specVersion = regexp.MustCompile(`^v\d+\.\d+$`)

// A Federation 1 subgraph has no @link: it uses the directives by their own names, and FieldSet as _FieldSet.
var federation1Link = specLink{localNames: map[string]string{
	"@key": "key", "@external": "external", "@requires": "requires", "@provides": "provides", "@extends": "extends", "@tag": "tag",
	"FieldSet": "_FieldSet",
}}

var typeSystemLocations = []ast.DirectiveLocation{
	ast.LocationSchema, ast.LocationScalar, ast.LocationObject, ast.LocationFieldDefinition, ast.LocationArgumentDefinition,
	ast.LocationInterface, ast.LocationUnion, ast.LocationEnum, ast.LocationEnumValue, ast.LocationInputObject,
	ast.LocationInputFieldDefinition,
}

// The subgraph adds _Any, _Entity, _Service and the Query fields _service and _entities for the router, and a library
// may print them; none is part of the API.
var federationOperationTypes = []string{"_Any", "_Entity", "_Service"}

const serviceField = "_service"

var federationOperationFields = []string{serviceField, "_entities"}

const serviceDefinitions = `type _Service { sdl: String }
`

// specLink is one @link: the namespace the elements of its spec take when not imported, and the name the SDL uses
// for each element it can use.
type specLink struct {
	namespace  string
	localNames map[string]string
}

// federation is what the @links of an SDL bring: the definitions to read it with, the federation links (or the
// Federation 1 definitions of a subgraph without one) and the links to other specs, the file of the first federation
// link, and what only the documents tell: by field coordinate, the @shareable of the type declaration that declares
// the field, and the @keys written on an extend type.
type federation struct {
	links             []specLink
	otherLinks        []specLink
	definitions       *ast.Source
	linkSource        string
	federation1Source string
	sharedByTheType   map[string]*ast.Directive
	keysOnExtension   map[string]bool
}

// A source that does not parse is left to LoadSchema, which reports it in its own words. A definition the SDL
// already declares, as federation-jvm prints them, is the SDL's own and never injected again. A subgraph without a
// federation @link is a Federation 1 subgraph.
func toFederation(sources []*ast.Source, inGraph bool) (federation, error) {
	var found federation
	definitions := &ast.SchemaDocument{}
	defined := map[string]bool{}
	accepted := map[string]bool{}

	var documents []*ast.SchemaDocument
	for _, source := range sources {
		document, err := parser.ParseSchema(source)
		if err != nil {
			continue
		}

		documents = append(documents, document)
		markDefined(document, defined)
	}

	for _, document := range documents {
		for _, directive := range schemaDirectives(document) {
			url, isLink := linkURLOf(directive)
			if !isLink {
				continue
			}

			if !strings.HasPrefix(url, federationURLPrefix) {
				found.otherLinks = append(found.otherLinks, toOtherLink(directive, url, accepted))

				continue
			}

			if version := strings.TrimPrefix(url, federationURLPrefix); !federation2Version.MatchString(version) {
				return federation{}, errorAtDirective(directive, "federation %s is not supported; supported: v2.x", version)
			}

			found.links = append(found.links, toFederationLink(directive, definitions, defined, accepted))
			if found.linkSource == "" {
				found.linkSource = directive.Position.Src.Name
			}
		}
	}

	if len(found.links) == 0 && inGraph && len(sources) > 0 {
		found.links = []specLink{federation1Link}
		found.federation1Source = sources[0].Name
		addUndefined(definitions, parseEmbedded(federation1Definitions), defined)
	}

	if len(found.links) == 0 && len(found.otherLinks) == 0 {
		return federation{}, nil
	}

	if found.linked() {
		addUndefined(definitions, parseEmbedded(linkDefinitions), defined)
	}

	addUndefined(definitions, toAcceptedDefinitions(documents, accepted, found.otherLinks), defined)
	if found.federated() {
		addServiceField(definitions, documents, defined)
	}

	found.sharedByTheType = toSharedByTheType(documents, found.localNamesOf("@"+graphqlmodel.ShareableDirective))
	found.keysOnExtension = toKeysOnExtension(documents, found.localNamesOf("@"+graphqlmodel.KeyDirective))

	var text bytes.Buffer
	formatter.NewFormatter(&text).FormatSchemaDocument(definitions)
	found.definitions = &ast.Source{Name: federationNamespace, Input: text.String(), BuiltIn: true}

	return found, nil
}

// Apollo adds _service to the query root of every subgraph, so a subgraph's query root may declare no field of its
// own. A query root the SDL does not define as an object, or that already prints _service, is left as written.
func addServiceField(definitions *ast.SchemaDocument, documents []*ast.SchemaDocument, defined map[string]bool) {
	root := queryRootName(documents)
	if !definesObject(documents, root) || declaresField(documents, root, serviceField) {
		return
	}

	addUndefined(definitions, parseEmbedded(serviceDefinitions), defined)
	definitions.Extensions = append(definitions.Extensions, &ast.Definition{Kind: ast.Object, Name: root, Fields: ast.FieldList{
		{Name: serviceField, Type: ast.NonNullNamedType("_Service", nil)},
	}})
}

func queryRootName(documents []*ast.SchemaDocument) string {
	for _, document := range documents {
		for _, schema := range slices.Concat(document.Schema, document.SchemaExtension) {
			for _, operation := range schema.OperationTypes {
				if operation.Operation == ast.Query {
					return operation.Type
				}
			}
		}
	}

	return "Query"
}

func definesObject(documents []*ast.SchemaDocument, name string) bool {
	return slices.ContainsFunc(documents, func(document *ast.SchemaDocument) bool {
		definition := document.Definitions.ForName(name)

		return definition != nil && definition.Kind == ast.Object
	})
}

func declaresField(documents []*ast.SchemaDocument, typeName, field string) bool {
	return slices.ContainsFunc(documents, func(document *ast.SchemaDocument) bool {
		return slices.ContainsFunc(slices.Concat(document.Definitions, document.Extensions), func(declaration *ast.Definition) bool {
			return declaration.Name == typeName && declaration.Fields.ForName(field) != nil
		})
	})
}

// gqlparser merges a type with its extensions, so only the documents tell which declaration a field and a @shareable
// come from.
func toSharedByTheType(documents []*ast.SchemaDocument, shareableNames []string) map[string]*ast.Directive {
	shared := map[string]*ast.Directive{}
	for _, document := range documents {
		for _, declaration := range slices.Concat(document.Definitions, document.Extensions) {
			for _, directive := range declaration.Directives {
				if !slices.Contains(shareableNames, directive.Name) {
					continue
				}

				for _, field := range declaration.Fields {
					shared[declaration.Name+"."+field.Name] = directive
				}
			}
		}
	}

	return shared
}

func schemaDirectives(document *ast.SchemaDocument) ast.DirectiveList {
	var directives ast.DirectiveList
	for _, schema := range document.Schema {
		directives = append(directives, schema.Directives...)
	}

	for _, extension := range document.SchemaExtension {
		directives = append(directives, extension.Directives...)
	}

	return directives
}

func linkURLOf(directive *ast.Directive) (string, bool) {
	if directive.Name != "link" {
		return "", false
	}

	url := directive.Arguments.ForName("url")
	if url == nil || url.Value.Kind != ast.StringValue {
		return "", false
	}

	return url.Value.Raw, true
}

// The definitions are read again for each link, so renaming them never touches another link's. An import the
// definitions lack is accepted, whatever it is.
func toFederationLink(directive *ast.Directive, definitions *ast.SchemaDocument, defined, accepted map[string]bool) specLink {
	specification := parseEmbedded(federationDefinitions)
	namespace := namespaceOf(directive, federationNamespace)

	localNames := map[string]string{}
	for _, definition := range specification.Directives {
		localNames["@"+definition.Name] = namespace + "__" + definition.Name
	}

	for _, definition := range specification.Definitions {
		localNames[definition.Name] = namespace + "__" + definition.Name
	}

	for _, imported := range importsOf(directive) {
		if _, defines := localNames[imported.name]; !defines {
			accepted[imported.localElement()] = true
		}

		localNames[imported.name] = imported.localName()
	}

	for _, definition := range specification.Definitions {
		definition.Name = localNames[definition.Name]
	}

	for _, definition := range specification.Directives {
		definition.Name = localNames["@"+definition.Name]
		for _, argument := range definition.Arguments {
			renameTypes(argument.Type, localNames)
		}
	}

	addUndefined(definitions, specification, defined)

	return specLink{namespace: namespace, localNames: localNames}
}

// A link to another spec, Connectors or a lone link, tag or inaccessible, brings definitions we don't know: every
// element it imports, and every one used through its namespace or named as it, is accepted and ignored.
func toOtherLink(directive *ast.Directive, url string, accepted map[string]bool) specLink {
	localNames := map[string]string{}
	for _, imported := range importsOf(directive) {
		accepted[imported.localElement()] = true
		localNames[imported.name] = imported.localName()
	}

	return specLink{namespace: namespaceOf(directive, specNameOf(url)), localNames: localNames}
}

// A spec URL ends in its name and version: https://specs.apollo.dev/connect/v0.1 is connect.
func specNameOf(url string) string {
	segments := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if last := len(segments) - 1; last > 0 && specVersion.MatchString(segments[last]) {
		return segments[last-1]
	}

	return segments[len(segments)-1]
}

func namespaceOf(directive *ast.Directive, spec string) string {
	if as := directive.Arguments.ForName("as"); as != nil && as.Value.Kind == ast.StringValue {
		return strings.TrimPrefix(as.Value.Raw, "@")
	}

	return spec
}

// An accepted directive takes whatever arguments the SDL gives it, anywhere and repeated; an accepted type is a
// scalar.
func toAcceptedDefinitions(documents []*ast.SchemaDocument, accepted map[string]bool, otherLinks []specLink) *ast.SchemaDocument {
	acceptedDefinitions := &ast.SchemaDocument{}
	for _, element := range slices.Sorted(maps.Keys(accepted)) {
		if !strings.HasPrefix(element, "@") {
			acceptedDefinitions.Definitions = append(acceptedDefinitions.Definitions, &ast.Definition{Kind: ast.Scalar, Name: element})
		}
	}

	arguments := map[string][]string{}
	for _, directive := range directiveUses(documents) {
		if !accepted["@"+directive.Name] && !slices.ContainsFunc(otherLinks, func(link specLink) bool {
			return directive.Name == link.namespace || strings.HasPrefix(directive.Name, link.namespace+"__")
		}) {
			continue
		}

		names := arguments[directive.Name]
		for _, argument := range directive.Arguments {
			if !slices.Contains(names, argument.Name) {
				names = append(names, argument.Name)
			}
		}

		arguments[directive.Name] = names
	}

	for _, name := range slices.Sorted(maps.Keys(arguments)) {
		definition := &ast.DirectiveDefinition{Name: name, Locations: typeSystemLocations, IsRepeatable: true}
		for _, argument := range arguments[name] {
			definition.Arguments = append(definition.Arguments, &ast.ArgumentDefinition{Name: argument, Type: ast.NamedType("String", nil)})
		}

		acceptedDefinitions.Directives = append(acceptedDefinitions.Directives, definition)
	}

	return acceptedDefinitions
}

func directiveUses(documents []*ast.SchemaDocument) ast.DirectiveList {
	var uses ast.DirectiveList
	for _, document := range documents {
		uses = append(uses, schemaDirectives(document)...)

		for _, declaration := range slices.Concat(document.Definitions, document.Extensions) {
			uses = append(uses, declaration.Directives...)

			for _, field := range declaration.Fields {
				uses = append(uses, field.Directives...)
				for _, argument := range field.Arguments {
					uses = append(uses, argument.Directives...)
				}
			}

			for _, value := range declaration.EnumValues {
				uses = append(uses, value.Directives...)
			}
		}

		for _, definition := range document.Directives {
			for _, argument := range definition.Arguments {
				uses = append(uses, argument.Directives...)
			}
		}
	}

	return uses
}

func parseEmbedded(content string) *ast.SchemaDocument {
	document, err := parser.ParseSchema(&ast.Source{Name: federationNamespace, Input: content})
	if err != nil {
		panic(err)
	}

	return document
}

func markDefined(document *ast.SchemaDocument, defined map[string]bool) {
	for _, definition := range document.Definitions {
		defined[definition.Name] = true
	}

	for _, definition := range document.Directives {
		defined["@"+definition.Name] = true
	}
}

func addUndefined(definitions, from *ast.SchemaDocument, defined map[string]bool) {
	for _, definition := range from.Definitions {
		if !defined[definition.Name] {
			definitions.Definitions = append(definitions.Definitions, definition)
		}
	}

	for _, definition := range from.Directives {
		if !defined["@"+definition.Name] {
			definitions.Directives = append(definitions.Directives, definition)
		}
	}

	markDefined(from, defined)
}

type federationImport struct {
	name string
	as   string
}

func (this federationImport) localName() string {
	return strings.TrimPrefix(this.as, "@")
}

// A directive is named with its "@", a type without.
func (this federationImport) localElement() string {
	if strings.HasPrefix(this.name, "@") {
		return "@" + this.localName()
	}

	return this.localName()
}

func importsOf(directive *ast.Directive) []federationImport {
	argument := directive.Arguments.ForName("import")
	if argument == nil || argument.Value.Kind != ast.ListValue {
		return nil
	}

	imports := make([]federationImport, 0, len(argument.Value.Children))
	for _, child := range argument.Value.Children {
		switch child.Value.Kind {
		case ast.StringValue:
			imports = append(imports, federationImport{name: child.Value.Raw, as: child.Value.Raw})
		case ast.ObjectValue:
			name := child.Value.Children.ForName("name")
			if name == nil {
				continue
			}

			imported := federationImport{name: name.Raw, as: name.Raw}
			if as := child.Value.Children.ForName("as"); as != nil {
				imported.as = as.Raw
			}

			imports = append(imports, imported)
		}
	}

	return imports
}

func renameTypes(fieldType *ast.Type, localNames map[string]string) {
	if fieldType.Elem != nil {
		renameTypes(fieldType.Elem, localNames)

		return
	}

	if localName, renamed := localNames[fieldType.NamedType]; renamed {
		fieldType.NamedType = localName
	}
}

// gqlparser places a directive at its name; the error points at its "@", one column before.
func errorAtDirective(directive *ast.Directive, message string, args ...any) error {
	return gqlerror.ErrorLocf(directive.Position.Src.Name, directive.Position.Line, directive.Position.Column-1, message, args...)
}

// countedDirectives maps the local name of each directive the composition counts to its federation name.
func (this federation) countedDirectives() map[string]string {
	counted := map[string]string{}
	for _, directive := range []string{graphqlmodel.KeyDirective, graphqlmodel.ShareableDirective, graphqlmodel.InaccessibleDirective, graphqlmodel.ExternalDirective, graphqlmodel.OverrideDirective, graphqlmodel.ProvidesDirective} {
		for _, name := range this.localNamesOf("@" + directive) {
			counted[name] = directive
		}
	}

	return counted
}

func (this federation) localNamesOf(element string) []string {
	var names []string
	for _, link := range this.links {
		if name, defines := link.localNames[element]; defines && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	return names
}

func (this federation) federated() bool {
	return len(this.links) > 0
}

func (this federation) linked() bool {
	return this.linkSource != "" || len(this.otherLinks) > 0
}

// The definitions the links bring, and the ones a subgraph library prints next to them, are no part of the API.
func (this federation) definesType(name string) bool {
	if this.federated() && slices.Contains(federationOperationTypes, name) {
		return true
	}

	if this.linked() && strings.HasPrefix(name, "link__") {
		return true
	}

	return slices.ContainsFunc(slices.Concat(this.links, this.otherLinks), func(link specLink) bool {
		if link.namespace != "" && strings.HasPrefix(name, link.namespace+"__") {
			return true
		}

		for element, localName := range link.localNames {
			if !strings.HasPrefix(element, "@") && localName == name {
				return true
			}
		}

		return false
	})
}

func (this federation) definesField(definition *ast.Definition, schema *ast.Schema, field string) bool {
	return this.federated() && definition == schema.Query && slices.Contains(federationOperationFields, field)
}

// gqlparser merges a type with its extensions, so only the documents tell which @key is written on an extend type.
func toKeysOnExtension(documents []*ast.SchemaDocument, keyNames []string) map[string]bool {
	keys := map[string]bool{}
	for _, document := range documents {
		for _, extension := range document.Extensions {
			for _, directive := range extension.Directives {
				if slices.Contains(keyNames, directive.Name) {
					keys[positionOf(directive)] = true
				}
			}
		}
	}

	return keys
}

func positionOf(directive *ast.Directive) string {
	return directive.Position.Src.Name + ":" + strconv.Itoa(directive.Position.Line) + ":" + strconv.Itoa(directive.Position.Column)
}

// A field is external when it or its object type is marked @external, as Apollo tells it, unless a @key written on
// an extend type, or on a type marked @extends, selects it: that @external only says the field is the key. A
// Federation 1 subgraph's type-level @external is dropped, as Apollo's upgrade drops it.
func (this federation) toExternalFields(schema *ast.Schema) map[string]bool {
	externalNames := this.localNamesOf("@" + graphqlmodel.ExternalDirective)
	keyNames := this.localNamesOf("@" + graphqlmodel.KeyDirective)
	extendsNames := this.localNamesOf("@extends")

	marked := func(directives ast.DirectiveList, names []string) bool {
		return slices.ContainsFunc(directives, func(directive *ast.Directive) bool { return slices.Contains(names, directive.Name) })
	}

	keyFields := map[string]bool{}
	var collect func(definition *ast.Definition, selections ast.SelectionSet)
	collect = func(definition *ast.Definition, selections ast.SelectionSet) {
		for _, selection := range selections {
			field := selection.(*ast.Field)
			declared := definition.Fields.ForName(field.Name)
			if declared == nil {
				continue
			}

			keyFields[definition.Name+"."+field.Name] = true
			if len(field.SelectionSet) > 0 {
				collect(schema.Types[declared.Type.Name()], field.SelectionSet)
			}
		}
	}

	for _, definition := range schema.Types {
		for _, key := range definition.Directives {
			if !slices.Contains(keyNames, key.Name) || !this.keysOnExtension[positionOf(key)] && !marked(definition.Directives, extendsNames) {
				continue
			}

			if selections, valid := toFieldSet(fieldsOf(key)); valid {
				collect(definition, selections)
			}
		}
	}

	external := map[string]bool{}
	for _, definition := range schema.Types {
		onType := this.federation1Source == "" && definition.Kind == ast.Object && marked(definition.Directives, externalNames)
		for _, field := range definition.Fields {
			coordinate := definition.Name + "." + field.Name
			onField := marked(field.Directives, externalNames)
			fake := onField && keyFields[coordinate]
			if (onType || onField) && !fake {
				external[coordinate] = true
			}
		}
	}

	return external
}
