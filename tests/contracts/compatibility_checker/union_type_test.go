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
	paymentConsumerName = "shop_web"
	paymentProviderName = "shop_api"
	paymentEnvironment  = "production"
	paymentProviderHash = "post-shop-api-orders"
	paymentEndpoint     = "/orders"
	paymentMethod       = "post"
)

type paymentsWithoutVerdict struct{}

func (this paymentsWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type paymentBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

var paymentMemberSchemas = model.Schemas{
	"CardPayment": {"": {Type: "object"}, "number": {Type: "string"}},
	"PixPayment":  {"": {Type: "object"}, "key": {Type: "string"}},
}

func paymentUnion(schema string, members ...string) model.Schemas {
	schemas := model.Schemas{schema: {"": {Type: "object"}, "payment": {Type: "union"}}}
	for _, member := range members {
		schemas[schema]["payment("+member+")"] = model.Field{Ref: map[string]string{"card": "CardPayment", "pix": "PixPayment"}[member]}
	}

	for name, fields := range paymentMemberSchemas {
		schemas[name] = fields
	}

	return schemas
}

func paymentResources(interaction string, consumerBodies []string, consumerSchemas, providerSchemas model.Schemas) (consumer, provider model.PersistedResource) {
	body, status := model.Output, interaction
	if interaction == "request" {
		body, status = model.Input, ""
	}

	var bodies []model.ResourceBody
	for _, schema := range consumerBodies {
		bodies = append(bodies, model.ResourceBody{Source: schema + ".yaml", SchemaName: null.StringFrom(schema)})
	}

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  paymentConsumerName,
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             body,
			Identity:         restmodel.Identity{Endpoint: paymentEndpoint, Method: paymentMethod, Status: status},
			ConsumedProvider: null.StringFrom(paymentProviderName),
			Bodies:           bodies,
		},
		ConsumerHash: null.StringFrom("shop-web-" + paymentProviderHash),
		ProviderHash: paymentProviderHash,
		Schemas:      consumerSchemas,
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: paymentProviderName,
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            body,
			Identity:        restmodel.Identity{Endpoint: paymentEndpoint, Method: paymentMethod, Status: status},
			Bodies:          []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("OrderRecord")}},
		},
		ProviderHash:       paymentProviderHash,
		Schemas:            providerSchemas,
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	return consumer, provider
}

func paymentBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []paymentBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(paymentsWithoutVerdict{}, []compatibility_checker.ProtocolHooks{restcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(paymentEnvironment),
		counterparts,
	)

	var breaks []paymentBreak
	for _, leaf := range report.Hierarchical[counterpartName].Trees[restmodel.Protocol].(restmodel.ReportTree)[paymentEndpoint][paymentMethod][interaction] {
		breaks = append(breaks, paymentBreak{leaf.Reason, leaf.Details})
	}

	return breaks
}

func assertPaymentBreaks(t *testing.T, interaction string, consumerBodies []string, consumerSchemas, providerSchemas model.Schemas, expected []paymentBreak) {
	t.Helper()

	consumer, provider := paymentResources(interaction, consumerBodies, consumerSchemas, providerSchemas)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, paymentBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{paymentProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{paymentProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, paymentProviderName, interaction))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, paymentBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{paymentProviderHash: {consumer}},
		}, paymentConsumerName, interaction))
	})
}

func TestUnionAgainstUnion(t *testing.T) {
	missingPix := []paymentBreak{{model.ReasonPropertyMemberMissingInProvider, map[string]string{
		"property":      "$.payment",
		"consumerName":  paymentConsumerName,
		"providerName":  paymentProviderName,
		"consumerField": "Order.payment",
		"providerField": "OrderRecord.payment",
		"member":        "pix",
	}}}

	for _, interaction := range []string{"request", "200"} {
		t.Run(interaction+": a member more in the provider", func(t *testing.T) {
			assertPaymentBreaks(t, interaction, []string{"Order"}, paymentUnion("Order", "card"), paymentUnion("OrderRecord", "card", "pix"), nil)
		})

		t.Run(interaction+": a consumer member the provider lacks", func(t *testing.T) {
			assertPaymentBreaks(t, interaction, []string{"Order"}, paymentUnion("Order", "card", "pix"), paymentUnion("OrderRecord", "card"), missingPix)
		})
	}
}

func TestUnionAgainstUnion_AFieldInsideACommonMemberBreaksAsAProperty(t *testing.T) {
	providerSchemas := paymentUnion("OrderRecord", "card")
	providerSchemas["CardPayment"] = model.Schema{"": {Type: "object"}, "digits": {Type: "string"}}

	assertPaymentBreaks(t, "200", []string{"Order"}, paymentUnion("Order", "card"), providerSchemas, []paymentBreak{{model.ReasonPropertyMissingInProvider, map[string]string{
		"property":      "$.payment(card).number",
		"consumerName":  paymentConsumerName,
		"providerName":  paymentProviderName,
		"consumerField": "CardPayment.number",
		"propertyType":  "string",
	}}})
}

func TestUnionSentByTwoConsumerBodies_JoinsTheirMembers(t *testing.T) {
	consumerSchemas := paymentUnion("OrderA", "card")
	for name, fields := range paymentUnion("OrderB", "card", "pix") {
		consumerSchemas[name] = fields
	}

	assertPaymentBreaks(t, "request", []string{"OrderA", "OrderB"}, consumerSchemas, paymentUnion("OrderRecord", "card", "pix"), nil)
}

func TestUnionAgainstAnotherTypeIsATypeMismatch(t *testing.T) {
	providerSchemas := model.Schemas{"OrderRecord": {"": {Type: "object"}, "payment": {Type: "string"}}}

	for _, interaction := range []string{"request", "200"} {
		t.Run(interaction, func(t *testing.T) {
			assertPaymentBreaks(t, interaction, []string{"Order"}, paymentUnion("Order", "card"), providerSchemas, []paymentBreak{{model.ReasonPropertyTypeMismatch, map[string]string{
				"property":             "$.payment",
				"consumerName":         paymentConsumerName,
				"providerName":         paymentProviderName,
				"consumerField":        "Order.payment",
				"providerField":        "OrderRecord.payment",
				"consumerPropertyType": "union",
				"providerPropertyType": "string",
			}}})
		})
	}
}
