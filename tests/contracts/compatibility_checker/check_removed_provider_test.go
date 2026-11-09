package compatibility_checker_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

var removedProviderHooks = []compatibility_checker.ProtocolHooks{restcontracts.New()}

func removedProviderChecker() *compatibility_checker.CompatibilityChecker {
	return compatibility_checker.NewCompatibilityChecker(nil, removedProviderHooks)
}

func removedProviderResource() *model.PersistedResource {
	return &model.PersistedResource{
		Resource: model.Resource{
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            model.Output,
			Identity:        restmodel.Identity{Endpoint: "/users", Method: "get", Status: "200"},
			ParticipantName: "users",
		},
		ProviderHash:       "provider-hash",
		ParticipantVersion: null.StringFrom("v2"),
		Removed:            true,
	}
}

func stillConsumingResource(participantID int64, participantName, version string) model.PersistedResource {
	return model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  participantName,
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             model.Output,
			Identity:         restmodel.Identity{Endpoint: "/users", Method: "get", Status: "200"},
			ConsumedProvider: null.StringFrom("users"),
		},
		ParticipantID:      participantID,
		ContractID:         participantID * 10,
		ProviderHash:       "provider-hash",
		ConsumerHash:       null.StringFrom("consumer-hash-" + participantName),
		ParticipantVersion: null.StringFrom(version),
	}
}

func TestRemovedProviderBreaksOncePerConsumer(t *testing.T) {
	removed := removedProviderResource()
	consumers := []model.PersistedResource{
		stillConsumingResource(3, "web", "v7"),
		stillConsumingResource(5, "mobile", "v2"),
	}

	results := removedProviderChecker().CheckRemovedProviderResource(removed, nil, consumers)

	assert.Len(t, results, 2)

	byConsumer := make(map[string]*compatibility_checker.CounterpartResult, len(results))
	for _, result := range results {
		byConsumer[result.Counterpart.ParticipantName] = result
	}

	web := byConsumer["web"]
	assert.False(t, web.Deployable)
	assert.Equal(t, int64(3), web.Counterpart.ParticipantID)
	assert.Equal(t, null.StringFrom("v7"), web.Counterpart.ParticipantVersion)
	assert.Zero(t, web.Counterpart.ContractID)

	assert.Len(t, web.Breaks, 1)
	assert.Equal(
		t,
		model.ReasonProviderResourceRemovedButStillConsumed,
		web.Breaks[0].Reason,
	)
	assert.Nil(t, web.Breaks[0].Details)
	assert.False(t, web.Breaks[0].IsPropertyBreak())
	assert.Same(t, removed, web.Breaks[0].CheckedResource)
	assert.Equal(t, "web", web.Breaks[0].CounterpartResource.ParticipantName)
	assert.Equal(t, "web", web.Breaks[0].ConsumerName())
	assert.Equal(t, "users", web.Breaks[0].ProviderName())
	assert.Equal(t, "200", web.Breaks[0].CheckedResource.Identity.(restmodel.Identity).Interaction())

	mobile := byConsumer["mobile"]
	assert.Equal(t, int64(5), mobile.Counterpart.ParticipantID)
	assert.Equal(t, null.StringFrom("v2"), mobile.Counterpart.ParticipantVersion)
	assert.Zero(t, mobile.Counterpart.ContractID)
	assert.Len(t, mobile.Breaks, 1)
	assert.Equal(t, "mobile", mobile.Breaks[0].CounterpartResource.ParticipantName)
}

func TestRemovedProviderWithoutConsumersHasNoBreak(t *testing.T) {
	results := removedProviderChecker().CheckRemovedProviderResource(
		removedProviderResource(),
		nil,
		nil,
	)

	assert.Empty(t, results)
}

func TestRemovedProviderResultKeepsTheRealContractIDInAnyOrder(t *testing.T) {
	removal := func() *compatibility_checker.CounterpartResult {
		results := removedProviderChecker().CheckRemovedProviderResource(
			removedProviderResource(),
			nil,
			[]model.PersistedResource{stillConsumingResource(3, "web", "v7")},
		)

		return results[0]
	}

	matched := func() *compatibility_checker.CounterpartResult {
		item := compatibility_checker.NewCounterpartResult()
		item.Counterpart = compatibility_checker.Counterpart{
			ParticipantID:      3,
			ContractID:         42,
			ParticipantName:    "web",
			ParticipantVersion: null.StringFrom("v7"),
		}

		return item
	}

	removalFirst := compatibility_checker.NewContractCompatibilityReport("users", "v2", "production", removedProviderHooks)
	removalFirst.AppendResult("web", removal())
	removalFirst.AppendResult("web", matched())

	matchedFirst := compatibility_checker.NewContractCompatibilityReport("users", "v2", "production", removedProviderHooks)
	matchedFirst.AppendResult("web", matched())
	matchedFirst.AppendResult("web", removal())

	for _, report := range []*compatibility_checker.ContractCompatibilityReport{removalFirst, matchedFirst} {
		result := report.Results["web"]

		assert.Equal(t, int64(42), result.Counterpart.ContractID)
		assert.Equal(t, int64(3), result.Counterpart.ParticipantID)
		assert.Equal(t, "web", result.Counterpart.ParticipantName)
		assert.Equal(t, null.StringFrom("v7"), result.Counterpart.ParticipantVersion)
		assert.Len(t, result.Breaks, 1)
	}
}

func liveUsersProviderResource(method string, body model.BodyRole, status string) model.PersistedResource {
	return model.PersistedResource{
		Resource: model.Resource{
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            body,
			Identity:        restmodel.Identity{Endpoint: "/users", Method: method, Status: status},
			ParticipantName: "users",
		},
		ParticipantVersion: null.StringFrom("v2"),
	}
}

func TestRemovedProvider_StatusChangeOnSurvivingMethod_BreaksAsInteractionRemovedWithSortedStatuses(t *testing.T) {
	removed := removedProviderResource()
	removedSibling := liveUsersProviderResource("get", model.Output, "500")
	removedSibling.Removed = true
	consumedSameMethod := stillConsumingResource(3, "web", "v7")
	consumedSameMethod.Identity = restmodel.Identity{Endpoint: "/users", Method: "get", Status: "201"}
	checkedResources := map[string]model.PersistedResource{
		"removed":      *removed,
		"404":          liveUsersProviderResource("get", model.Output, "404"),
		"204":          liveUsersProviderResource("get", model.Output, "204"),
		"2XX":          liveUsersProviderResource("get", model.Output, "2XX"),
		"request":      liveUsersProviderResource("get", model.Input, ""),
		"other method": liveUsersProviderResource("post", model.Output, "201"),
		"removed 500":  removedSibling,
		"consumed 201": consumedSameMethod,
	}

	results := removedProviderChecker().CheckRemovedProviderResource(
		removed,
		checkedResources,
		[]model.PersistedResource{stillConsumingResource(3, "web", "v7")},
	)

	assert.Len(t, results, 1)
	web := results[0]
	assert.False(t, web.Deployable)
	assert.Zero(t, web.Counterpart.ContractID)
	assert.Len(t, web.Breaks, 1)
	assert.Equal(
		t,
		model.ReasonProviderResourceInteractionRemovedButStillConsumed,
		web.Breaks[0].Reason,
	)
	assert.Equal(t, map[string]string{"providerStatuses": "204, 2XX, 404"}, web.Breaks[0].Details)
	assert.False(t, web.Breaks[0].IsPropertyBreak())
	assert.Same(t, removed, web.Breaks[0].CheckedResource)
	assert.Equal(t, "web", web.Breaks[0].ConsumerName())
	assert.Equal(t, "200", web.Breaks[0].CheckedResource.Identity.(restmodel.Identity).Interaction())
}

func TestRemovedProvider_RequestDroppedOnSurvivingMethod_BreaksAsInteractionRemovedWithoutDetails(t *testing.T) {
	removed := removedProviderResource()
	removed.Body = model.Input
	removed.Identity = restmodel.Identity{Endpoint: "/users", Method: "get"}
	consumer := stillConsumingResource(3, "web", "v7")
	consumer.Body = model.Input
	consumer.Identity = restmodel.Identity{Endpoint: "/users", Method: "get"}
	checkedResources := map[string]model.PersistedResource{
		"removed": *removed,
		"201":     liveUsersProviderResource("get", model.Output, "201"),
	}

	results := removedProviderChecker().CheckRemovedProviderResource(
		removed,
		checkedResources,
		[]model.PersistedResource{consumer},
	)

	assert.Len(t, results, 1)
	assert.Len(t, results[0].Breaks, 1)
	assert.Equal(
		t,
		model.ReasonProviderResourceInteractionRemovedButStillConsumed,
		results[0].Breaks[0].Reason,
	)
	assert.Nil(t, results[0].Breaks[0].Details)
	assert.Equal(t, "request", results[0].Breaks[0].CheckedResource.Identity.(restmodel.Identity).Interaction())
	assert.False(t, results[0].Breaks[0].IsPropertyBreak())
}

func TestRemovedProvider_ResponseDroppedWhereOnlyARequestRemains_BreaksAsInteractionRemovedWithoutDetails(t *testing.T) {
	removed := removedProviderResource()
	checkedResources := map[string]model.PersistedResource{
		"removed": *removed,
		"request": liveUsersProviderResource("get", model.Input, ""),
	}

	results := removedProviderChecker().CheckRemovedProviderResource(
		removed,
		checkedResources,
		[]model.PersistedResource{stillConsumingResource(3, "web", "v7")},
	)

	assert.Len(t, results, 1)
	assert.Len(t, results[0].Breaks, 1)
	assert.Equal(
		t,
		model.ReasonProviderResourceInteractionRemovedButStillConsumed,
		results[0].Breaks[0].Reason,
	)
	assert.Nil(t, results[0].Breaks[0].Details)
}

func TestRemovedProvider_MethodGoneWhileEndpointRemains_KeepsTheRemovedReason(t *testing.T) {
	removed := removedProviderResource()
	consumedSameMethod := stillConsumingResource(3, "web", "v7")
	consumedSameMethod.Identity = restmodel.Identity{Endpoint: "/users", Method: "get", Status: "204"}
	removedSibling := liveUsersProviderResource("get", model.Output, "204")
	removedSibling.Removed = true
	checkedResources := map[string]model.PersistedResource{
		"removed":      *removed,
		"removed 204":  removedSibling,
		"other method": liveUsersProviderResource("post", model.Output, "201"),
		"consumed 204": consumedSameMethod,
	}

	results := removedProviderChecker().CheckRemovedProviderResource(
		removed,
		checkedResources,
		[]model.PersistedResource{stillConsumingResource(3, "web", "v7")},
	)

	assert.Len(t, results, 1)
	assert.Len(t, results[0].Breaks, 1)
	assert.Equal(
		t,
		model.ReasonProviderResourceRemovedButStillConsumed,
		results[0].Breaks[0].Reason,
	)
	assert.Nil(t, results[0].Breaks[0].Details)
}
