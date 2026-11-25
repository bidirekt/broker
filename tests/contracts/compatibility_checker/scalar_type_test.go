package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

const (
	bornAtConsumerName = "petstore_web"
	bornAtProviderName = "petstore_api"
	bornAtEnvironment  = "production"
	bornAtProviderHash = "petstore_api-query-pet-returns"
)

type bornAtWithoutVerdict struct{}

func (this bornAtWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type bornAtBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

func bornAtPet(bornAt model.Field) model.Schema {
	return model.Schema{"": {Type: "object"}, "bornAt": bornAt}
}

func bornAtResources(consumerBornAt, providerBornAt model.Field) (consumer, provider model.PersistedResource) {
	identity := graphqlmodel.Identity{Operation: graphqlmodel.QueryOperation, Field: "pet", Interaction: graphqlmodel.ReturnsInteraction}

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  bornAtConsumerName,
			Direction:        model.Consumes,
			Protocol:         graphqlmodel.Protocol,
			Body:             model.Output,
			Identity:         identity,
			ConsumedProvider: null.StringFrom(bornAtProviderName),
			Bodies:           []model.ResourceBody{{Source: "petstore_web.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("Pet")}},
		},
		ConsumerHash: null.StringFrom("petstore_web-" + bornAtProviderHash),
		ProviderHash: bornAtProviderHash,
		Schemas:      map[string]model.Schemas{model.YAMLNamespace: {"Pet": bornAtPet(consumerBornAt)}},
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: bornAtProviderName,
			Direction:       model.Provides,
			Protocol:        graphqlmodel.Protocol,
			Body:            model.Output,
			Identity:        identity,
			Bodies:          []model.ResourceBody{{Source: "petstore_api.graphql", Namespace: graphqlmodel.Namespace, SchemaName: null.StringFrom("Query:pet:returns")}},
		},
		ProviderHash:       bornAtProviderHash,
		Schemas:            map[string]model.Schemas{graphqlmodel.Namespace: {"Query:pet:returns": bornAtPet(providerBornAt)}},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	return consumer, provider
}

func bornAtBreaksCheckedFrom(checked model.PersistedResource, counterparts model.ResourceCounterparts, counterpartName string) []bornAtBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(bornAtWithoutVerdict{}, []compatibility_checker.ProtocolHooks{graphqlcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(bornAtEnvironment),
		counterparts,
	)

	var breaks []bornAtBreak
	for _, found := range report.Results[counterpartName].Breaks {
		breaks = append(breaks, bornAtBreak{found.Reason, found.Details})
	}

	return breaks
}

func assertBornAtBreaks(t *testing.T, consumerBornAt, providerBornAt model.Field, expected []bornAtBreak) {
	t.Helper()

	consumer, provider := bornAtResources(consumerBornAt, providerBornAt)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, bornAtBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{bornAtProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{bornAtProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, bornAtProviderName))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, bornAtBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{bornAtProviderHash: {consumer}},
		}, bornAtConsumerName))
	})
}

func bornAtTypeMismatch(consumerType, providerType string) []bornAtBreak {
	return []bornAtBreak{{model.ReasonPropertyTypeMismatch, map[string]string{
		"property":             "$.bornAt",
		"consumerName":         bornAtConsumerName,
		"providerName":         bornAtProviderName,
		"consumerPropertyType": consumerType,
		"providerPropertyType": providerType,
		"consumerField":        "Pet.bornAt",
		"providerField":        "Query.pet.bornAt",
	}}}
}

func TestScalarType_TheSameScalar_IsCompatible(t *testing.T) {
	assertBornAtBreaks(t,
		model.Field{Type: "scalar", Scalar: "DateTime"},
		model.Field{Type: "scalar", Scalar: "DateTime"},
		nil)
}

func TestScalarType_AnotherScalar_IsATypeMismatch(t *testing.T) {
	assertBornAtBreaks(t,
		model.Field{Type: "scalar", Scalar: "DateTime"},
		model.Field{Type: "scalar", Scalar: "Date"},
		bornAtTypeMismatch("DateTime", "Date"))
}

func TestScalarType_ANativeType_IsATypeMismatch(t *testing.T) {
	assertBornAtBreaks(t,
		model.Field{Type: "scalar", Scalar: "DateTime"},
		model.Field{Type: "string"},
		bornAtTypeMismatch("DateTime", "string"))
}
