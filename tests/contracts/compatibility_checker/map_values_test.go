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
	reason   model.BreakingReason
	property string
}

func petsMapProvidedByTheProvider() model.Schema {
	return model.Schema{
		"":            {Type: "object"},
		"pets":        {Type: "map"},
		"pets{}":      {Type: "object"},
		"pets{}.name": {Type: "string"},
	}
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

func assertPetsBreaks(t *testing.T, readByTheConsumer, providedByTheProvider model.Schema, expected []petsBreak) {
	t.Helper()

	consumer := model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:    petsConsumerName,
			Direction:          model.Consumes,
			Interaction:        model.RestResponse,
			ConsumedProvider:   null.StringFrom(petsProviderName),
			Endpoint:           "/api/pets",
			Method:             "get",
			ResponseStatusCode: null.StringFrom("200"),
			Bodies:             []model.ResourceBody{{Source: "spa.yaml", SchemaName: null.StringFrom("Pets")}},
		},
		ConsumerHash: null.StringFrom("spa-" + petsProviderHash),
		ProviderHash: petsProviderHash,
		Schemas:      model.Schemas{"Pets": readByTheConsumer},
	}
	provider := model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:    petsProviderName,
			Direction:          model.Provides,
			Interaction:        model.RestResponse,
			Endpoint:           "/api/pets",
			Method:             "get",
			ResponseStatusCode: null.StringFrom("200"),
			Bodies:             []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pets")}},
		},
		ProviderHash:       petsProviderHash,
		Schemas:            model.Schemas{"Pets": providedByTheProvider},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, petsBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{petsProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{petsProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
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
	readByTheConsumer := model.Schema{
		"":       {Type: "object"},
		"pets":   {Type: "map"},
		"pets{}": {Type: "string"},
	}

	assertPetsBreaks(t, readByTheConsumer, petsMapProvidedByTheProvider(), []petsBreak{
		{model.ReasonPropertyTypeMismatch, "$.pets{}"},
	})
}

func TestNullableMapValuesBreakAgainstANonNullableReader(t *testing.T) {
	providedByTheProvider := petsMapProvidedByTheProvider()
	providedByTheProvider["pets{}"] = model.Field{Type: "object", Nullable: true}

	assertPetsBreaks(t, petsMapProvidedByTheProvider(), providedByTheProvider, []petsBreak{
		{model.ReasonPropertyNullableInProviderNotNullableInConsumer, "$.pets{}"},
	})
}

func TestMapValueChildMissingInTheProviderBreaksOnTheChild(t *testing.T) {
	readByTheConsumer := petsMapProvidedByTheProvider()
	readByTheConsumer["pets{}.age"] = model.Field{Type: "integer"}

	assertPetsBreaks(t, readByTheConsumer, petsMapProvidedByTheProvider(), []petsBreak{
		{model.ReasonPropertyMissingInProvider, "$.pets{}.age"},
	})
}

func TestMapValueMemberWithAnotherTypeBreaksOnTheMember(t *testing.T) {
	readByTheConsumer := petsMapProvidedByTheProvider()
	readByTheConsumer["pets{}.name"] = model.Field{Type: "integer"}

	assertPetsBreaks(t, readByTheConsumer, petsMapProvidedByTheProvider(), []petsBreak{
		{model.ReasonPropertyTypeMismatch, "$.pets{}.name"},
	})
}

func TestObjectReadAgainstAMapBreaksOnTheParent(t *testing.T) {
	readByTheConsumer := model.Schema{
		"":         {Type: "object"},
		"pets":     {Type: "object"},
		"pets.bob": {Type: "object"},
	}

	assertPetsBreaks(t, readByTheConsumer, petsMapProvidedByTheProvider(), []petsBreak{
		{model.ReasonPropertyTypeMismatch, "$.pets"},
	})
}

func TestMapReadAgainstAnObjectBreaksOnTheParent(t *testing.T) {
	providedByTheProvider := model.Schema{
		"":              {Type: "object"},
		"pets":          {Type: "object"},
		"pets.bob":      {Type: "object"},
		"pets.bob.name": {Type: "string"},
	}

	assertPetsBreaks(t, petsMapProvidedByTheProvider(), providedByTheProvider, []petsBreak{
		{model.ReasonPropertyTypeMismatch, "$.pets"},
	})
}
