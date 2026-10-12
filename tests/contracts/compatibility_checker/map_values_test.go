package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

const (
	petsConsumerName = "spa"
	petsProviderName = "api"
	petsEnvironment  = "production"
	petsProviderHash = "get-api-pets"
)

type petsWithoutVerdict struct{}

func (this petsWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type petsBreak struct {
	reason   compatibility_checker.BreakingReason
	property string
}

func petsProperties(properties ...model.Property) map[string]model.Property {
	byPath := make(map[string]model.Property, len(properties))
	for _, property := range properties {
		byPath[property.Path] = property
	}

	return byPath
}

func petsMapProvidedByTheProvider() map[string]model.Property {
	return petsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.pets", Type: "object"},
		model.Property{Path: "$.pets{}", Type: "object"},
		model.Property{Path: "$.pets{}.name", Type: "string"},
	)
}

func petsBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName string,
) []petsBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(petsWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(petsEnvironment),
		counterparts,
	)

	var breaks []petsBreak
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints["/api/pets"]["get"]["200"] {
		breaks = append(breaks, petsBreak{leaf.Reason, leaf.Details["property"]})
	}

	return breaks
}

func assertPetsBreaks(t *testing.T, readByTheConsumer, providedByTheProvider map[string]model.Property, expected []petsBreak) {
	t.Helper()

	consumer := model.PersistedResource{
		ParticipantName:    petsConsumerName,
		Direction:          model.Consumes,
		Interaction:        model.RestResponse,
		ConsumedProvider:   null.StringFrom(petsProviderName),
		ConsumerHash:       null.StringFrom("spa-" + petsProviderHash),
		ProviderHash:       petsProviderHash,
		Endpoint:           "/api/pets",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		Properties:         readByTheConsumer,
	}
	provider := model.PersistedResource{
		ParticipantName:    petsProviderName,
		Direction:          model.Provides,
		Interaction:        model.RestResponse,
		ProviderHash:       petsProviderHash,
		Endpoint:           "/api/pets",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		Properties:         providedByTheProvider,
		DeployedVersions:   map[string]string{petsEnvironment: "1.0.0"},
	}

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, petsBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers: map[string]model.PersistedResource{petsProviderHash: provider},
		}, petsProviderName))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, petsBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{petsProviderHash: {consumer}},
		}, petsConsumerName))
	})
}

func TestMapValuesReadAsProvidedHaveNoBreak(t *testing.T) {
	assertPetsBreaks(t, petsMapProvidedByTheProvider(), petsMapProvidedByTheProvider(), nil)
}

func TestMapValuesWithAnotherTypeBreakOnTheValues(t *testing.T) {
	readByTheConsumer := petsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.pets", Type: "object"},
		model.Property{Path: "$.pets{}", Type: "string"},
	)

	assertPetsBreaks(t, readByTheConsumer, petsMapProvidedByTheProvider(), []petsBreak{
		{compatibility_checker.ReasonPropertyTypeMismatch, "$.pets{}"},
	})
}

func TestOptionalMapValuesBreakAgainstARequiredReader(t *testing.T) {
	providedByTheProvider := petsMapProvidedByTheProvider()
	providedByTheProvider["$.pets{}"] = model.Property{Path: "$.pets{}", Type: "object", Optional: true}

	assertPetsBreaks(t, petsMapProvidedByTheProvider(), providedByTheProvider, []petsBreak{
		{compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, "$.pets{}"},
	})
}

func TestNullableMapValuesBreakAgainstANonNullableReader(t *testing.T) {
	providedByTheProvider := petsMapProvidedByTheProvider()
	providedByTheProvider["$.pets{}"] = model.Property{Path: "$.pets{}", Type: "object", Nullable: true}

	assertPetsBreaks(t, petsMapProvidedByTheProvider(), providedByTheProvider, []petsBreak{
		{compatibility_checker.ReasonPropertyNullableInProviderNotNullableInConsumer, "$.pets{}"},
	})
}

func TestMapValueChildMissingInTheProviderBreaksOnTheChild(t *testing.T) {
	readByTheConsumer := petsMapProvidedByTheProvider()
	readByTheConsumer["$.pets{}.age"] = model.Property{Path: "$.pets{}.age", Type: "integer"}

	assertPetsBreaks(t, readByTheConsumer, petsMapProvidedByTheProvider(), []petsBreak{
		{compatibility_checker.ReasonPropertyMissingInProvider, "$.pets{}.age"},
	})
}

func TestFixedKeyReadAgainstAMapIsMissingInTheProvider(t *testing.T) {
	readByTheConsumer := petsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.pets", Type: "object"},
		model.Property{Path: "$.pets.bob", Type: "object"},
	)
	providedByTheProvider := petsProperties(
		model.Property{Path: "$", Type: "object"},
		model.Property{Path: "$.pets", Type: "object"},
		model.Property{Path: "$.pets{}", Type: "object"},
	)

	assertPetsBreaks(t, readByTheConsumer, providedByTheProvider, []petsBreak{
		{compatibility_checker.ReasonPropertyMissingInProvider, "$.pets.bob"},
	})
}
