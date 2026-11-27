package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func TestMergeExtensions_JoinsTheTypeInfoOfEveryFile_SortedByCoordinate(t *testing.T) {
	merged, violations := graphqlcontracts.New().MergeExtensions([]protocol.ExtensionPiece{
		{Source: "pet.graphqls", Value: graphqlmodel.Extension{Types: []graphqlmodel.TypeInfo{
			{Coordinate: "Pet", Kind: "object"},
			{Coordinate: "Pet.id", Kind: "field", NamedType: "ID"},
		}}},
		{Source: "schema.graphqls", Value: graphqlmodel.Extension{Types: []graphqlmodel.TypeInfo{
			{Coordinate: "Owner", Kind: "object"},
			{Coordinate: "Query.pet(id:)", Kind: "argument", NamedType: "ID"},
		}}},
	})

	assert.Empty(t, violations)
	assert.Equal(t, graphqlmodel.Extension{Types: []graphqlmodel.TypeInfo{
		{Coordinate: "Owner", Kind: "object"},
		{Coordinate: "Pet", Kind: "object"},
		{Coordinate: "Pet.id", Kind: "field", NamedType: "ID"},
		{Coordinate: "Query.pet(id:)", Kind: "argument", NamedType: "ID"},
	}}, merged)
}

func TestMergeExtensions_TheSameSDLSplitIntoOtherFiles_MergesTheSame(t *testing.T) {
	whole, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{
		{Source: "schema.graphqls", Content: "type Query {\n  pet(id: ID!): Pet\n}\n\ntype Pet {\n  id: ID!\n}\n"},
	})
	require.NoError(t, readErr)

	split, readErr := graphqlcontracts.NewSDLFormat().ToDeclarations([]model.ContractFile{
		{Source: "a.graphqls", Content: "type Pet {\n  id: ID!\n}\n"},
		{Source: "b.graphqls", Content: "type Query {\n  pet(id: ID!): Pet\n}\n"},
	})
	require.NoError(t, readErr)

	wholeMerged, _ := graphqlcontracts.New().MergeExtensions(whole.ExtensionPieces["graphql"])
	splitMerged, _ := graphqlcontracts.New().MergeExtensions(split.ExtensionPieces["graphql"])
	assert.Equal(t, wholeMerged, splitMerged)
}
