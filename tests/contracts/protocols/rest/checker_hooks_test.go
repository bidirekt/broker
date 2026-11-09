package rest_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/stretchr/testify/assert"
)

func hooksUsersProvider(method string, body model.BodyRole, status string) model.PersistedResource {
	return model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: "users",
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            body,
			Identity:        restmodel.Identity{Endpoint: "/users", Method: method, Status: status},
		},
	}
}

func TestPluginIsTheCheckerHooksOfRest(t *testing.T) {
	var hooks compatibility_checker.ProtocolHooks = restcontracts.New()

	tree := hooks.NewReportTree()

	assert.Equal(t, restmodel.Protocol, hooks.Protocol())
	assert.Equal(t, restmodel.ReportTree{}, tree)
	assert.Equal(t, "endpoints", tree.ReportKey())
}

func TestRefineNotFound(t *testing.T) {
	consumer := hooksUsersProvider("get", model.Output, "200")
	consumer.Direction = model.Consumes

	cases := []struct {
		name    string
		hint    any
		reason  model.BreakingReason
		details map[string]string
		refined bool
	}{
		{"statuses declared on the method", restmodel.DeployedStatuses{"201", "204"}, model.ReasonProviderResourceInteractionNotFound, map[string]string{"providerStatuses": "201, 204"}, true},
		{"only a request declared on the method", restmodel.DeployedStatuses(nil), model.ReasonProviderResourceInteractionNotFound, nil, true},
		{"a hint of another protocol", []string{"204"}, "", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, details, refined := restcontracts.New().RefineNotFound(&consumer, tc.hint)

			assert.Equal(t, tc.reason, reason)
			assert.Equal(t, tc.details, details)
			assert.Equal(t, tc.refined, refined)
		})
	}
}

func TestRefineRemoved_ResponseOnSurvivingMethod_ListsTheLiveProviderStatusesSorted(t *testing.T) {
	removed := hooksUsersProvider("get", model.Output, "200")
	removed.Removed = true
	removedSibling := hooksUsersProvider("get", model.Output, "500")
	removedSibling.Removed = true
	consumed := hooksUsersProvider("get", model.Output, "201")
	consumed.Direction = model.Consumes

	reason, details, refined := restcontracts.New().RefineRemoved(&removed, map[string]model.PersistedResource{
		"removed":        removed,
		"404":            hooksUsersProvider("get", model.Output, "404"),
		"204":            hooksUsersProvider("get", model.Output, "204"),
		"2XX":            hooksUsersProvider("get", model.Output, "2XX"),
		"request":        hooksUsersProvider("get", model.Input, ""),
		"other method":   hooksUsersProvider("post", model.Output, "201"),
		"other endpoint": {Resource: model.Resource{Direction: model.Provides, Protocol: restmodel.Protocol, Body: model.Output, Identity: restmodel.Identity{Endpoint: "/orders", Method: "get", Status: "202"}}},
		"removed 500":    removedSibling,
		"consumed 201":   consumed,
	})

	assert.True(t, refined)
	assert.Equal(t, model.ReasonProviderResourceInteractionRemovedButStillConsumed, reason)
	assert.Equal(t, map[string]string{"providerStatuses": "204, 2XX, 404"}, details)
}

func TestRefineRemoved_RequestOnSurvivingMethod_HasNoStatuses(t *testing.T) {
	removed := hooksUsersProvider("get", model.Input, "")
	removed.Removed = true

	reason, details, refined := restcontracts.New().RefineRemoved(&removed, map[string]model.PersistedResource{
		"removed": removed,
		"201":     hooksUsersProvider("get", model.Output, "201"),
	})

	assert.True(t, refined)
	assert.Equal(t, model.ReasonProviderResourceInteractionRemovedButStillConsumed, reason)
	assert.Nil(t, details)
}

func TestRefineRemoved_MethodGone_IsNotRefined(t *testing.T) {
	removed := hooksUsersProvider("get", model.Output, "200")
	removed.Removed = true

	reason, details, refined := restcontracts.New().RefineRemoved(&removed, map[string]model.PersistedResource{
		"removed":      removed,
		"other method": hooksUsersProvider("post", model.Output, "201"),
	})

	assert.False(t, refined)
	assert.Empty(t, reason)
	assert.Nil(t, details)
}
