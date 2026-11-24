package graphql

import (
	"bytes"
	"cmp"
	"embed"
	"path"
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

//go:embed federation/v*.graphql
var federationFiles embed.FS

var federationVersions = toFederationVersions()

// federationLink is one @link to a federation version, with the name the SDL uses for each element it can use.
type federationLink struct {
	localNames map[string]string
}

// federation is what the @links of an SDL bring: the definitions to read it with, the local name of each element
// and the file of the first link.
type federation struct {
	links       []federationLink
	definitions *ast.Source
	linkSource  string
}

func toFederationVersions() []string {
	entries, err := federationFiles.ReadDir("federation")
	if err != nil {
		panic(err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		versions = append(versions, strings.TrimSuffix(entry.Name(), ".graphql"))
	}

	slices.SortFunc(versions, func(a, b string) int {
		return cmp.Compare(minorOf(a), minorOf(b))
	})

	return versions
}

func minorOf(version string) int {
	minor, _ := strconv.Atoi(strings.TrimPrefix(version, "v2."))

	return minor
}

// A source that does not parse is left to LoadSchema, which reports it in its own words.
func toFederation(sources []*ast.Source) (federation, error) {
	var found federation
	definitions := &ast.SchemaDocument{}
	defined := map[string]bool{}

	for _, source := range sources {
		document, err := parser.ParseSchema(source)
		if err != nil {
			continue
		}

		for _, directive := range schemaDirectives(document) {
			version, isFederation := federationVersionOf(directive)
			if !isFederation {
				continue
			}

			link, linkErr := toFederationLink(directive, version, definitions, defined)
			if linkErr != nil {
				return federation{}, linkErr
			}

			found.links = append(found.links, link)
			if found.linkSource == "" {
				found.linkSource = source.Name
			}
		}
	}

	if len(found.links) == 0 {
		return federation{}, nil
	}

	var text bytes.Buffer
	text.WriteString(linkDefinitions)
	formatter.NewFormatter(&text).FormatSchemaDocument(definitions)
	found.definitions = &ast.Source{Name: federationNamespace, Input: text.String(), BuiltIn: true}

	return found, nil
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

func federationVersionOf(directive *ast.Directive) (string, bool) {
	if directive.Name != "link" {
		return "", false
	}

	url := directive.Arguments.ForName("url")
	if url == nil || url.Value.Kind != ast.StringValue || !strings.HasPrefix(url.Value.Raw, federationURLPrefix) {
		return "", false
	}

	return strings.TrimPrefix(url.Value.Raw, federationURLPrefix), true
}

// The definitions of a version are read again for each link, so renaming them never touches another link's.
func toFederationLink(directive *ast.Directive, version string, definitions *ast.SchemaDocument, defined map[string]bool) (federationLink, error) {
	if !slices.Contains(federationVersions, version) {
		return federationLink{}, errorAtDirective(directive, "federation %s is not supported; supported: %s", version, strings.Join(federationVersions, ", "))
	}

	content, err := federationFiles.ReadFile(path.Join("federation", version+".graphql"))
	if err != nil {
		panic(err)
	}

	specification, err := parser.ParseSchema(&ast.Source{Name: federationNamespace, Input: string(content)})
	if err != nil {
		panic(err)
	}

	namespace := federationNamespace
	if as := directive.Arguments.ForName("as"); as != nil && as.Value.Kind == ast.StringValue {
		namespace = strings.TrimPrefix(as.Value.Raw, "@")
	}

	localNames := map[string]string{}
	for _, definition := range specification.Directives {
		localNames["@"+definition.Name] = namespace + "__" + definition.Name
	}

	for _, definition := range specification.Definitions {
		localNames[definition.Name] = namespace + "__" + definition.Name
	}

	for _, imported := range importsOf(directive) {
		if _, defines := localNames[imported.name]; !defines {
			return federationLink{}, errorAtDirective(directive, "@link imports %s, which federation %s doesn't define", imported.name, version)
		}

		localNames[imported.name] = strings.TrimPrefix(imported.as, "@")
	}

	for _, definition := range specification.Definitions {
		definition.Name = localNames[definition.Name]
		if !defined[definition.Name] {
			defined[definition.Name] = true
			definitions.Definitions = append(definitions.Definitions, definition)
		}
	}

	for _, definition := range specification.Directives {
		definition.Name = localNames["@"+definition.Name]
		for _, argument := range definition.Arguments {
			renameTypes(argument.Type, localNames)
		}

		if !defined["@"+definition.Name] {
			defined["@"+definition.Name] = true
			definitions.Directives = append(definitions.Directives, definition)
		}
	}

	return federationLink{localNames: localNames}, nil
}

type federationImport struct {
	name string
	as   string
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
	for _, directive := range []string{graphqlmodel.KeyDirective, graphqlmodel.ShareableDirective, graphqlmodel.InaccessibleDirective, graphqlmodel.ExternalDirective} {
		for _, name := range this.localNamesOf("@" + directive) {
			counted[name] = directive
		}
	}

	return counted
}

func (this federation) localNamesOf(element string) []string {
	var names []string
	for _, link := range this.links {
		if name := link.localNames[element]; !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	return names
}
