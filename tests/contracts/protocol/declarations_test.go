package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
)

func TestAddSchema_ALaterSchemaOfTheSameName_IsListedButTheCatalogKeepsTheFirst(t *testing.T) {
	first := protocol.SchemaDeclaration{Source: "pet.graphqls", Namespace: "graphql", Name: "Pet", Schema: contract.Schema{Type: "object"}}
	later := protocol.SchemaDeclaration{Source: "schema.graphqls", Namespace: "graphql", Name: "Pet", Schema: contract.Schema{Type: "string"}}

	var declarations protocol.Declarations
	declarations.AddSchema(first)
	declarations.AddSchema(later)

	assert.Equal(t, []protocol.SchemaDeclaration{first, later}, declarations.Schemas)
	assert.Equal(t, map[string]contract.SchemasMap{"graphql": {"Pet": {Type: "object"}}}, declarations.Catalog)
}

func TestAddSchema_TheSameNameInAnotherNamespace_IsAnEntryOfItsOwn(t *testing.T) {
	var declarations protocol.Declarations
	declarations.AddSchema(protocol.SchemaDeclaration{Source: "pets.yaml", Namespace: "yaml", Name: "Pet", Schema: contract.Schema{Type: "object"}})
	declarations.AddSchema(protocol.SchemaDeclaration{Source: "schema.graphqls", Namespace: "graphql", Name: "Pet", Schema: contract.Schema{Type: "string"}})

	assert.Equal(t, map[string]contract.SchemasMap{
		"yaml":    {"Pet": {Type: "object"}},
		"graphql": {"Pet": {Type: "string"}},
	}, declarations.Catalog)
}

func TestAddExtensionPiece_EachProtocolKeepsItsPiecesInTheOrderAdded(t *testing.T) {
	var declarations protocol.Declarations
	declarations.AddExtensionPiece("graphql", protocol.ExtensionPiece{Source: "schema.graphqls", Value: "schema"})
	declarations.AddExtensionPiece("probe", protocol.ExtensionPiece{Source: "api.yaml", Value: "api"})
	declarations.AddExtensionPiece("graphql", protocol.ExtensionPiece{Source: "pet.graphqls", Value: "pet"})

	assert.Equal(t, map[string][]protocol.ExtensionPiece{
		"graphql": {{Source: "schema.graphqls", Value: "schema"}, {Source: "pet.graphqls", Value: "pet"}},
		"probe":   {{Source: "api.yaml", Value: "api"}},
	}, declarations.ExtensionPieces)
}
