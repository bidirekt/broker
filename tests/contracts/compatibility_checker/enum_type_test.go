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
	statusConsumerName = "petstore_web"
	statusProviderName = "petstore_api"
	statusEnvironment  = "production"
	statusProviderHash = "post-petstore-api-pets"
	statusEndpoint     = "/pets"
	statusMethod       = "post"
)

type statusesWithoutVerdict struct{}

func (this statusesWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type statusBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

func statusString() model.Field {
	return model.Field{Type: "string"}
}

func statusEnum(members ...string) model.Field {
	return model.Field{Type: "enum", Members: members}
}

func statusResources(interaction string, consumerStatus, providerStatus model.Field) (consumer, provider model.PersistedResource) {
	body, status := model.Output, interaction
	if interaction == "request" {
		body, status = model.Input, ""
	}

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  statusConsumerName,
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             body,
			Identity:         restmodel.Identity{Endpoint: statusEndpoint, Method: statusMethod, Status: status},
			ConsumedProvider: null.StringFrom(statusProviderName),
			Bodies:           []model.ResourceBody{{Source: "web.yaml", SchemaName: null.StringFrom("Pet")}},
		},
		ConsumerHash: null.StringFrom("petstore-web-" + statusProviderHash),
		ProviderHash: statusProviderHash,
		Schemas:      model.Schemas{"Pet": {"": {Type: "object"}, "status": consumerStatus}},
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: statusProviderName,
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            body,
			Identity:        restmodel.Identity{Endpoint: statusEndpoint, Method: statusMethod, Status: status},
			Bodies:          []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("PetRecord")}},
		},
		ProviderHash:       statusProviderHash,
		Schemas:            model.Schemas{"PetRecord": {"": {Type: "object"}, "status": providerStatus}},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	return consumer, provider
}

func statusBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []statusBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(statusesWithoutVerdict{}, []compatibility_checker.ProtocolHooks{restcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(statusEnvironment),
		counterparts,
	)

	var breaks []statusBreak
	for _, leaf := range report.Hierarchical[counterpartName].Trees[restmodel.Protocol].(restmodel.ReportTree)[statusEndpoint][statusMethod][interaction] {
		breaks = append(breaks, statusBreak{leaf.Reason, leaf.Details})
	}

	return breaks
}

func assertStatusBreaks(t *testing.T, interaction string, consumerStatus, providerStatus model.Field, expected []statusBreak) {
	t.Helper()

	consumer, provider := statusResources(interaction, consumerStatus, providerStatus)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, statusBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{statusProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{statusProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, statusProviderName, interaction))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, statusBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{statusProviderHash: {consumer}},
		}, statusConsumerName, interaction))
	})
}

func statusTypeMismatch(consumerType, providerType string) []statusBreak {
	return []statusBreak{{model.ReasonPropertyTypeMismatch, map[string]string{
		"property":             "$.status",
		"consumerName":         statusConsumerName,
		"providerName":         statusProviderName,
		"consumerField":        "Pet.status",
		"providerField":        "PetRecord.status",
		"consumerPropertyType": consumerType,
		"providerPropertyType": providerType,
	}}}
}

func statusMemberMissing(members ...string) []statusBreak {
	var breaks []statusBreak
	for _, member := range members {
		breaks = append(breaks, statusBreak{model.ReasonPropertyMemberMissingInProvider, map[string]string{
			"property":      "$.status",
			"consumerName":  statusConsumerName,
			"providerName":  statusProviderName,
			"consumerField": "Pet.status",
			"providerField": "PetRecord.status",
			"member":        member,
		}})
	}

	return breaks
}

func TestEnumAgainstStringOrEnum(t *testing.T) {
	cases := []struct {
		name        string
		interaction string
		consumer    model.Field
		provider    model.Field
		expected    []statusBreak
	}{
		{"request: enum sent, string read", "request", statusEnum("a"), statusString(), nil},
		{"request: string sent, enum read", "request", statusString(), statusEnum("a", "b"), statusTypeMismatch("string", "enum")},
		{"request: a value the provider lacks", "request", statusEnum("a", "c"), statusEnum("a", "b"), statusMemberMissing("c")},
		{"request: values within the provider's", "request", statusEnum("a"), statusEnum("a", "b"), nil},
		{"response: string provided, enum read", "200", statusEnum("a"), statusString(), statusTypeMismatch("enum", "string")},
		{"response: enum provided, string read", "200", statusString(), statusEnum("a", "b"), nil},
		{"response: a value the provider lacks", "200", statusEnum("a", "c"), statusEnum("a", "b"), statusMemberMissing("c")},
		{"response: values within the provider's", "200", statusEnum("a"), statusEnum("a", "b"), nil},
		{"response: one break per value the provider lacks", "200", statusEnum("a", "c", "d"), statusEnum("a"), statusMemberMissing("c", "d")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStatusBreaks(t, tc.interaction, tc.consumer, tc.provider, tc.expected)
		})
	}
}

func TestEnumAgainstAnotherTypeIsATypeMismatch(t *testing.T) {
	for _, interaction := range []string{"request", "200"} {
		t.Run(interaction, func(t *testing.T) {
			assertStatusBreaks(t, interaction, statusEnum("a"), model.Field{Type: "integer"}, statusTypeMismatch("enum", "integer"))
		})
	}
}
