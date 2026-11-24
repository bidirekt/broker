package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
)

func TestPlugin_Protocol_IsGraphQL(t *testing.T) {
	assert.Equal(t, model.Protocol("graphql"), graphqlcontracts.New().Protocol())
}

func TestNewReportTree_IsAnEmptyOperationsTree(t *testing.T) {
	tree := graphqlcontracts.New().NewReportTree()

	assert.Equal(t, "operations", tree.ReportKey())
	assert.True(t, tree.IsEmpty())
}
