package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

var (
	petArguments = map[string]string{"operation": "query", "field": "pet", "interaction": "arguments"}
	petReturns   = map[string]string{"operation": "query", "field": "pet", "interaction": "returns"}
)

func TestReportTree_ReportKey_IsOperations(t *testing.T) {
	assert.Equal(t, "operations", graphqlmodel.ReportTree{}.ReportKey())
}

func TestReportTree_IsEmpty_UntilABreakIsAppended(t *testing.T) {
	tree := graphqlmodel.ReportTree{}
	assert.True(t, tree.IsEmpty())

	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonPropertyTypeMismatch, Role: model.RoleConsumer, Details: map[string]string{"property": "$.age"}})

	assert.False(t, tree.IsEmpty())
}

func TestReportTree_Append_IndexesByOperationFieldAndInteraction(t *testing.T) {
	tree := graphqlmodel.ReportTree{}

	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonPropertyTypeMismatch, Role: model.RoleConsumer, Details: map[string]string{"property": "$.name"}})
	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer, Details: map[string]string{"property": "$.reviews", "argument": "first"}})
	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer, Details: map[string]string{"property": "$.reviews", "argument": "filter.minStars"}})
	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer, Details: map[string]string{"property": "$.owner"}})

	assert.Equal(t, graphqlmodel.ReportTree{"query": {"pet": {
		"arguments": {
			{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer, Details: map[string]string{"property": "$.owner"}},
		},
		"returns": {
			{Reason: model.ReasonPropertyTypeMismatch, Role: model.RoleConsumer, Details: map[string]string{"property": "$.name"}},
			{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer, Details: map[string]string{"property": "$.reviews", "argument": "filter.minStars"}},
			{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleConsumer, Details: map[string]string{"property": "$.reviews", "argument": "first"}},
		},
	}}}, tree)
}

func TestReportTree_Append_KeepsAResourceBreakOnceUnderReturns_WhenArgumentsArrivesFirst(t *testing.T) {
	tree := graphqlmodel.ReportTree{}

	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer})
	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer})

	assert.Equal(t, graphqlmodel.ReportTree{"query": {"pet": {"returns": {
		{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer},
	}}}}, tree)
}

func TestReportTree_Append_KeepsAResourceBreakOnceUnderReturns_WhenReturnsArrivesFirst(t *testing.T) {
	tree := graphqlmodel.ReportTree{}

	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer})
	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer})

	assert.Equal(t, graphqlmodel.ReportTree{"query": {"pet": {"returns": {
		{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer},
	}}}}, tree)
}

func TestReportTree_Append_KeepsResourceBreaksOfDifferentReasons(t *testing.T) {
	tree := graphqlmodel.ReportTree{}

	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer})
	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider})

	assert.Equal(t, graphqlmodel.ReportTree{"query": {"pet": {"returns": {
		{Reason: model.ReasonProviderResourceNotFound, Role: model.RoleConsumer},
		{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider},
	}}}}, tree)
}

func TestReportTree_Append_KeepsAResourceBreakWithOtherDetails_DirectFirst(t *testing.T) {
	tree := graphqlmodel.ReportTree{}

	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider, Details: map[string]string{"graph": "petstore_graph"}})
	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider})
	tree.Append(petReturns, model.ReportBreak{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider, Details: map[string]string{"graph": "petstore_graph"}})
	tree.Append(petArguments, model.ReportBreak{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider})

	assert.Equal(t, graphqlmodel.ReportTree{"query": {"pet": {"returns": {
		{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider},
		{Reason: model.ReasonProviderResourceRemovedButStillConsumed, Role: model.RoleProvider, Details: map[string]string{"graph": "petstore_graph"}},
	}}}}, tree)
}
