package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
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

func TestFieldName_OfAnArgument_IsItsGraphQLCoordinate(t *testing.T) {
	for path, expected := range map[string]string{
		"id":         "Query.pet(id:)",
		"ids[]":      "Query.pet(ids:)[]",
		"filter.max": "Query.pet(filter:).max",
	} {
		name, named := graphqlcontracts.New().FieldName(schemagraph.Site{Namespace: "graphql", Schema: "Query:pet:arguments", Path: path})

		assert.True(t, named, path)
		assert.Equal(t, expected, name, path)
	}
}

func TestFieldName_OfANestedFieldArgument_IsItsGraphQLCoordinate(t *testing.T) {
	name, named := graphqlcontracts.New().FieldName(schemagraph.Site{Namespace: "graphql", Schema: "Pet:reviews:arguments", Path: "first"})

	assert.True(t, named)
	assert.Equal(t, "Pet.reviews(first:)", name)
}

func TestFieldName_OfAnyOtherSite_KeepsTheSchemaName(t *testing.T) {
	for _, site := range []schemagraph.Site{
		{Namespace: "graphql", Schema: "Query:pet:returns", Path: "name"},
		{Namespace: "graphql", Schema: "Pet", Path: "name"},
		{Namespace: "graphql", Schema: "Query:pet:arguments"},
		{Namespace: "yaml", Schema: "Query:pet:arguments", Path: "id"},
	} {
		_, named := graphqlcontracts.New().FieldName(site)

		assert.False(t, named, site)
	}
}
