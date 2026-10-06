package compatibility_checker_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

func removedProviderResource() *model.PersistedResource {
	return &model.PersistedResource{
		Direction:          model.Provides,
		Interaction:        model.RestResponse,
		Endpoint:           "/users",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		ParticipantName:    "users",
		ProviderHash:       "provider-hash",
		ParticipantVersion: null.StringFrom("v2"),
		Removed:            true,
	}
}

func stillConsumingResource(participantID int64, participantName, version string) model.PersistedResource {
	return model.PersistedResource{
		ParticipantID:      participantID,
		ParticipantName:    participantName,
		ContractID:         participantID * 10,
		Direction:          model.Consumes,
		Interaction:        model.RestResponse,
		ConsumedProvider:   null.StringFrom("users"),
		Endpoint:           "/users",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
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

	results := compatibility_checker.CheckRemovedProviderResource(removed, consumers)

	assert.Len(t, results, 2)

	byConsumer := make(map[string]*compatibility_checker.IncompatibleItem, len(results))
	for _, result := range results {
		byConsumer[result.IncompatibleCounterpart.ParticipantName] = result
	}

	web := byConsumer["web"]
	assert.False(t, web.Deployable)
	assert.Equal(t, int64(3), web.IncompatibleCounterpart.ParticipantID)
	assert.Equal(t, null.StringFrom("v7"), web.IncompatibleCounterpart.ParticipantVersion)
	assert.Zero(t, web.IncompatibleCounterpart.ContractID)

	assert.Len(t, web.Breaks, 1)
	assert.Equal(
		t,
		compatibility_checker.ReasonProviderResourceRemovedButStillConsumed,
		web.Breaks[0].Reason,
	)
	assert.Nil(t, web.Breaks[0].Details)
	assert.False(t, web.Breaks[0].IsPropertyBreak())
	assert.Same(t, removed, web.Breaks[0].CheckedResource)
	assert.Equal(t, "web", web.Breaks[0].CounterpartResource.ParticipantName)
	assert.Equal(t, "web", web.Breaks[0].ConsumerName())
	assert.Equal(t, "users", web.Breaks[0].ProviderName())
	assert.Equal(t, "200", web.Breaks[0].InteractionKey())

	mobile := byConsumer["mobile"]
	assert.Equal(t, int64(5), mobile.IncompatibleCounterpart.ParticipantID)
	assert.Equal(t, null.StringFrom("v2"), mobile.IncompatibleCounterpart.ParticipantVersion)
	assert.Zero(t, mobile.IncompatibleCounterpart.ContractID)
	assert.Len(t, mobile.Breaks, 1)
	assert.Equal(t, "mobile", mobile.Breaks[0].CounterpartResource.ParticipantName)
}

func TestRemovedProviderWithoutConsumersHasNoBreak(t *testing.T) {
	results := compatibility_checker.CheckRemovedProviderResource(
		removedProviderResource(),
		nil,
	)

	assert.Empty(t, results)
}

func TestRemovedProviderResultKeepsTheRealContractIDInAnyOrder(t *testing.T) {
	removal := func() *compatibility_checker.IncompatibleItem {
		results := compatibility_checker.CheckRemovedProviderResource(
			removedProviderResource(),
			[]model.PersistedResource{stillConsumingResource(3, "web", "v7")},
		)

		return results[0]
	}

	matched := func() *compatibility_checker.IncompatibleItem {
		item := compatibility_checker.NewIncompatibleItem()
		item.IncompatibleCounterpart = compatibility_checker.IncompatibleCounterpart{
			ParticipantID:      3,
			ContractID:         42,
			ParticipantName:    "web",
			ParticipantVersion: null.StringFrom("v7"),
		}

		return item
	}

	removalFirst := compatibility_checker.NewContractCompatibilityReport("users", "v2", "production")
	removalFirst.AppendResult("web", removal())
	removalFirst.AppendResult("web", matched())

	matchedFirst := compatibility_checker.NewContractCompatibilityReport("users", "v2", "production")
	matchedFirst.AppendResult("web", matched())
	matchedFirst.AppendResult("web", removal())

	for _, report := range []*compatibility_checker.ContractCompatibilityReport{removalFirst, matchedFirst} {
		result := report.Results["web"]

		assert.Equal(t, int64(42), result.IncompatibleCounterpart.ContractID)
		assert.Equal(t, int64(3), result.IncompatibleCounterpart.ParticipantID)
		assert.Equal(t, "web", result.IncompatibleCounterpart.ParticipantName)
		assert.Equal(t, null.StringFrom("v7"), result.IncompatibleCounterpart.ParticipantVersion)
		assert.Len(t, result.Breaks, 1)
	}
}
