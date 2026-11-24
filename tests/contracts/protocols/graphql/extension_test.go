package graphql_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/violation"
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

func TestMergeExtensions_TheSameGraphInSeveralFiles_IsOneGraphFromTheFirstFile(t *testing.T) {
	merged, violations := graphqlcontracts.New().MergeExtensions([]protocol.ExtensionPiece{
		{Source: "api.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph"}},
		{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph"}},
	})

	assert.Empty(t, violations)
	assert.Equal(t, "petstore_graph", merged.(graphqlmodel.Extension).Graph)
	assert.Equal(t, "api.yaml", merged.(graphqlmodel.Extension).GraphSource)
}

func TestMergeExtensions_TwoGraphs_ConflictAtTheSecondFile(t *testing.T) {
	merged, violations := graphqlcontracts.New().MergeExtensions([]protocol.ExtensionPiece{
		{Source: "api.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph"}},
		{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "orders_graph"}},
	})

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "graph.conflict",
		Path:      "provides;graphql;graph",
		Source:    "graph.yaml",
		Details:   map[string]any{"graph": "orders_graph", "declaredIn": "api.yaml", "declared": "petstore_graph"},
	}}, violations)
	assert.Equal(t, "petstore_graph", merged.(graphqlmodel.Extension).Graph)
}

func TestMergeExtensions_TheSourcesStayOutOfTheJSON(t *testing.T) {
	merged, _ := graphqlcontracts.New().MergeExtensions([]protocol.ExtensionPiece{
		{Source: "graph.yaml", Value: graphqlmodel.Extension{Graph: "petstore_graph"}},
		{Source: "schema.graphqls", Value: graphqlmodel.Extension{LinkSource: "schema.graphqls"}},
	})

	encoded, err := json.Marshal(merged)
	require.NoError(t, err)

	assert.JSONEq(t, `{"graph":"petstore_graph"}`, string(encoded))
}
