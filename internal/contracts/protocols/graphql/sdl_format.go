package graphql

import (
	"fmt"
	"slices"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"
	"github.com/vektah/gqlparser/v2/parser"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
)

const formatName = "graphql"

type SDLFormat struct{}

func NewSDLFormat() SDLFormat {
	return SDLFormat{}
}

func (this SDLFormat) Extensions() []string {
	return []string{".graphql", ".graphqls", ".gql"}
}

// All the files are one schema: a type defined in one file may be used or extended in another.
func (this SDLFormat) ToDeclarations(files []model.ContractFile) (protocol.Declarations, error) {
	sources := make([]*ast.Source, 0, len(files))

	for _, file := range files {
		source := &ast.Source{Name: file.Source, Input: file.Content}
		if err := refuseClientOperations(source); err != nil {
			return protocol.Declarations{}, err
		}

		sources = append(sources, source)
	}

	federation, err := toFederation(sources)
	if err != nil {
		return protocol.Declarations{}, err
	}

	schema, err := gqlparser.LoadSchema(withDefinitions(sources, federation.definitions)...)
	if err != nil {
		return protocol.Declarations{}, err
	}

	if err := checkExtendedTypesDefined(sources); err != nil {
		return protocol.Declarations{}, err
	}

	if err := federation.checkKeys(schema, sources); err != nil {
		return protocol.Declarations{}, err
	}

	return toDeclarations(schema, federation), nil
}

// gqlparser takes the extension of a type no file defines as its definition; graphql-js refuses it.
func checkExtendedTypesDefined(sources []*ast.Source) error {
	document, err := parser.ParseSchemas(sources...)
	if err != nil {
		return err
	}

	defined := map[string]bool{}
	for _, definition := range document.Definitions {
		defined[definition.Name] = true
	}

	for _, extension := range document.Extensions {
		if !defined[extension.Name] {
			return gqlerror.ErrorPosf(extension.Position, "Cannot extend type %q because it is not defined.", extension.Name)
		}
	}

	return nil
}

var executableKeywords = []string{"query", "mutation", "subscription", "fragment"}

func refuseClientOperations(source *ast.Source) error {
	if !startsWithExecutableDefinition(source) {
		return nil
	}

	if _, err := parser.ParseQuery(source); err != nil {
		return fmt.Errorf("%w in client operations; publish expects the GraphQL schema", err)
	}

	return fmt.Errorf("%s: holds client operations; publish expects the GraphQL schema", source.Name)
}

func startsWithExecutableDefinition(source *ast.Source) bool {
	scanner := lexer.New(source)

	token := readTokenSkippingComments(&scanner)
	if isDescription(token) {
		token = readTokenSkippingComments(&scanner)
	}

	return token.Kind == lexer.BraceL || token.Kind == lexer.Name && slices.Contains(executableKeywords, token.Value)
}

func readTokenSkippingComments(scanner *lexer.Lexer) lexer.Token {
	for {
		token, err := scanner.ReadToken()
		if err != nil {
			return lexer.Token{Kind: lexer.Invalid}
		}

		if token.Kind != lexer.Comment {
			return token
		}
	}
}

func isDescription(token lexer.Token) bool {
	return token.Kind == lexer.String || token.Kind == lexer.BlockString
}

func withDefinitions(sources []*ast.Source, definitions *ast.Source) []*ast.Source {
	if definitions == nil {
		return sources
	}

	return append([]*ast.Source{definitions}, sources...)
}
