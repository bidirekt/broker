package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func TestReportTree_ReportKey_IsOperations(t *testing.T) {
	assert.Equal(t, "operations", graphqlmodel.ReportTree{}.ReportKey())
}

func TestReportTree_StaysEmpty(t *testing.T) {
	tree := graphqlmodel.ReportTree{}

	tree.Append(map[string]string{}, model.ReportBreak{Reason: model.ReasonProviderResourceNotFound})

	assert.True(t, tree.IsEmpty())
}
