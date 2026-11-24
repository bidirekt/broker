package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

func TestPlugin_Protocol_IsGraphQL(t *testing.T) {
	assert.Equal(t, model.Protocol("graphql"), graphqlcontracts.New().Protocol())
}

func TestGrammar_AnEmptyBlock_IsValidOnBothSides(t *testing.T) {
	grammar := protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}).Grammar()

	assert.Empty(t, descriptor.Validate(grammar, map[string]any{
		"provides": map[string]any{"graphql": map[string]any{}},
		"consumes": map[string]any{"pets_api": map[string]any{"graphql": nil}},
	}, "api.yaml"))
}

func TestGrammar_AnyKeyInTheBlock_IsUnknownOnBothSides(t *testing.T) {
	grammar := protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}).Grammar()

	violations := descriptor.Validate(grammar, map[string]any{
		"provides": map[string]any{"graphql": map[string]any{"graph": "pets"}},
		"consumes": map[string]any{"pets_api": map[string]any{"graphql": map[string]any{"query": map[string]any{}}}},
	}, "api.yaml")

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "key.unknown", Path: "consumes;pets_api;graphql;query", Source: "api.yaml", Details: map[string]string{"key": "query"}},
		{ErrorCode: "key.unknown", Path: "provides;graphql;graph", Source: "api.yaml", Details: map[string]string{"key": "graph"}},
	}, violations)
}

func TestToDeclarations_DeclaresNothing(t *testing.T) {
	assert.Empty(t, graphqlcontracts.New().ToDeclarations(protocol.Block{Document: map[string]any{}}))
}

func TestNewReportTree_IsAnEmptyOperationsTree(t *testing.T) {
	tree := graphqlcontracts.New().NewReportTree()

	assert.Equal(t, "operations", tree.ReportKey())
	assert.True(t, tree.IsEmpty())
}
