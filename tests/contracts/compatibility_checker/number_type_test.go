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
	pricesConsumerName = "storefront"
	pricesProviderName = "catalog"
	pricesEnvironment  = "production"
	pricesProviderHash = "put-catalog-prices"
	pricesEndpoint     = "/prices"
	pricesMethod       = "put"
)

type pricesWithoutVerdict struct{}

func (this pricesWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type pricesBreak struct {
	reason   compatibility_checker.BreakingReason
	property string
}

func pricesBody(amountType string) ([]model.ResourceBody, model.Schemas) {
	bodies := []model.ResourceBody{{Source: "prices.yaml", SchemaName: null.StringFrom("Price")}}

	return bodies, model.Schemas{"Price": {
		"":       {Type: "object"},
		"amount": {Type: amountType},
	}}
}

func pricesResources(interaction, consumerType, providerType string) (consumer, provider model.PersistedResource) {
	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumerBodies, consumerSchemas := pricesBody(consumerType)
	providerBodies, providerSchemas := pricesBody(providerType)

	consumer = model.PersistedResource{
		ParticipantName:    pricesConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(pricesProviderName),
		ConsumerHash:       null.StringFrom("storefront-" + pricesProviderHash),
		ProviderHash:       pricesProviderHash,
		Endpoint:           pricesEndpoint,
		Method:             pricesMethod,
		ResponseStatusCode: status,
		Bodies:             consumerBodies,
		Schemas:            consumerSchemas,
	}

	provider = model.PersistedResource{
		ParticipantName:    pricesProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       pricesProviderHash,
		Endpoint:           pricesEndpoint,
		Method:             pricesMethod,
		ResponseStatusCode: status,
		Bodies:             providerBodies,
		Schemas:            providerSchemas,
		DeployedVersions:   map[string]string{pricesEnvironment: "1.0.0"},
	}

	return consumer, provider
}

func pricesBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []pricesBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(pricesWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(pricesEnvironment),
		counterparts,
	)

	var breaks []pricesBreak
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints[pricesEndpoint][pricesMethod][interaction] {
		breaks = append(breaks, pricesBreak{leaf.Reason, leaf.Details["property"]})
	}

	return breaks
}

func assertPricesBreaks(t *testing.T, interaction, consumerType, providerType string, expected []pricesBreak) {
	t.Helper()

	consumer, provider := pricesResources(interaction, consumerType, providerType)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, pricesBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers: map[string]model.PersistedResource{pricesProviderHash: provider},
		}, pricesProviderName, interaction))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, pricesBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{pricesProviderHash: {consumer}},
		}, pricesConsumerName, interaction))
	})
}

var amountTypeMismatch = []pricesBreak{{compatibility_checker.ReasonPropertyTypeMismatch, "$.amount"}}

var numberReaderCases = []struct {
	producer string
	reader   string
	expected []pricesBreak
}{
	{"integer", "number", nil},
	{"float", "number", nil},
	{"number", "number", nil},
	{"number", "integer", amountTypeMismatch},
	{"number", "float", amountTypeMismatch},
	{"integer", "float", amountTypeMismatch},
	{"float", "integer", amountTypeMismatch},
}

func TestNumberConsumerReadsAnIntegerOrFloatResponse(t *testing.T) {
	for _, tc := range numberReaderCases {
		t.Run(tc.producer+" provided, "+tc.reader+" read", func(t *testing.T) {
			assertPricesBreaks(t, "200", tc.reader, tc.producer, tc.expected)
		})
	}
}

func TestNumberProviderReadsAnIntegerOrFloatRequest(t *testing.T) {
	for _, tc := range numberReaderCases {
		t.Run(tc.producer+" sent, "+tc.reader+" read", func(t *testing.T) {
			assertPricesBreaks(t, "request", tc.producer, tc.reader, tc.expected)
		})
	}
}
