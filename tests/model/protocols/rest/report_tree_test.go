package rest_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/stretchr/testify/assert"
)

func TestReportTree_ReportKey_IsEndpoints(t *testing.T) {
	assert.Equal(t, "endpoints", restmodel.ReportTree{}.ReportKey())
}

func TestReportTree_Append_IndexesByEndpointMethodAndInteraction(t *testing.T) {
	tree := restmodel.ReportTree{}
	location := map[string]string{"endpoint": "/pets", "method": "get", "interaction": "200"}

	tree.Append(location, model.ReportBreak{Reason: model.ReasonPropertyTypeMismatch, Role: model.RoleProvider, Details: map[string]string{"property": "name"}})
	tree.Append(location, model.ReportBreak{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleProvider, Details: map[string]string{"property": "name"}})
	tree.Append(location, model.ReportBreak{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleProvider, Details: map[string]string{"property": "id"}})

	assert.Equal(t, restmodel.ReportTree{"/pets": {"get": {"200": {
		{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleProvider, Details: map[string]string{"property": "id"}},
		{Reason: model.ReasonPropertyMissingInProvider, Role: model.RoleProvider, Details: map[string]string{"property": "name"}},
		{Reason: model.ReasonPropertyTypeMismatch, Role: model.RoleProvider, Details: map[string]string{"property": "name"}},
	}}}}, tree)
}
