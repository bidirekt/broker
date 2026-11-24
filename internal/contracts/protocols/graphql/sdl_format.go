package graphql

import (
	"fmt"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
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
	return []string{".graphql", ".graphqls"}
}

// All the files are one schema: a type defined in one file may be used or extended in another.
func (this SDLFormat) ToDeclarations(files []model.ContractFile) (protocol.Declarations, error) {
	sources := make([]*ast.Source, 0, len(files))

	for _, file := range files {
		source := &ast.Source{Name: file.Source, Input: file.Content}
		if holdsOperations(source) {
			return protocol.Declarations{}, fmt.Errorf("%s: holds operations; publish expects the GraphQL schema", file.Source)
		}

		sources = append(sources, source)
	}

	schema, err := gqlparser.LoadSchema(sources...)
	if err != nil {
		return protocol.Declarations{}, err
	}

	return toDeclarations(schema), nil
}

func holdsOperations(source *ast.Source) bool {
	document, err := parser.ParseQuery(source)

	return err == nil && len(document.Operations) > 0
}
