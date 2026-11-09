package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
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
	reason   model.BreakingReason
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
	body, status := model.Output, interaction
	if interaction == "request" {
		body, status = model.Input, ""
	}

	consumerBodies, consumerSchemas := pricesBody(consumerType)
	providerBodies, providerSchemas := pricesBody(providerType)

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  pricesConsumerName,
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             body,
			Identity:         restmodel.Identity{Endpoint: pricesEndpoint, Method: pricesMethod, Status: status},
			ConsumedProvider: null.StringFrom(pricesProviderName),
			Bodies:           consumerBodies,
		},
		ConsumerHash: null.StringFrom("storefront-" + pricesProviderHash),
		ProviderHash: pricesProviderHash,
		Schemas:      consumerSchemas,
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: pricesProviderName,
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            body,
			Identity:        restmodel.Identity{Endpoint: pricesEndpoint, Method: pricesMethod, Status: status},
			Bodies:          providerBodies,
		},
		ProviderHash:       pricesProviderHash,
		Schemas:            providerSchemas,
		ParticipantVersion: null.StringFrom("1.0.0"),
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

	report := compatibility_checker.NewCompatibilityChecker(pricesWithoutVerdict{}, []compatibility_checker.ProtocolHooks{restcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(pricesEnvironment),
		counterparts,
	)

	var breaks []pricesBreak
	for _, leaf := range report.Hierarchical[counterpartName].Trees[restmodel.Protocol].(restmodel.ReportTree)[pricesEndpoint][pricesMethod][interaction] {
		breaks = append(breaks, pricesBreak{leaf.Reason, leaf.Details["property"]})
	}

	return breaks
}

func assertPricesBreaks(t *testing.T, interaction, consumerType, providerType string, expected []pricesBreak) {
	t.Helper()

	consumer, provider := pricesResources(interaction, consumerType, providerType)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, pricesBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{pricesProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{pricesProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, pricesProviderName, interaction))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, pricesBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{pricesProviderHash: {consumer}},
		}, pricesConsumerName, interaction))
	})
}

var amountTypeMismatch = []pricesBreak{{model.ReasonPropertyTypeMismatch, "$.amount"}}

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
