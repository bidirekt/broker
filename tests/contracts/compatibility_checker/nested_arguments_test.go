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
	reviewsConsumerName = "petstore_web"
	reviewsProviderName = "petstore_api"
	reviewsEnvironment  = "production"
	reviewsProviderHash = "petstore_api-query-pet-returns"
)

type reviewsWithoutVerdict struct{}

func (this reviewsWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type reviewsBreak struct {
	reason  model.BreakingReason
	details map[string]string
}

func reviewsResources(consumerSchemas, providerSchemas model.Schemas) (consumer, provider model.PersistedResource) {
	identity := graphqlmodel.Identity{Operation: graphqlmodel.QueryOperation, Field: "pet", Interaction: graphqlmodel.ReturnsInteraction}

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  reviewsConsumerName,
			Direction:        model.Consumes,
			Protocol:         graphqlmodel.Protocol,
			Body:             model.Output,
			Identity:         identity,
			ConsumedProvider: null.StringFrom(reviewsProviderName),
			Bodies:           []model.ResourceBody{{Source: "petstore_web.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("Pet")}},
		},
		ConsumerHash: null.StringFrom("petstore_web-" + reviewsProviderHash),
		ProviderHash: reviewsProviderHash,
		Schemas:      map[string]model.Schemas{model.YAMLNamespace: consumerSchemas},
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: reviewsProviderName,
			Direction:       model.Provides,
			Protocol:        graphqlmodel.Protocol,
			Body:            model.Output,
			Identity:        identity,
			Bodies:          []model.ResourceBody{{Source: "petstore_api.graphql", Namespace: graphqlmodel.Namespace, SchemaName: null.StringFrom("Query:pet:returns")}},
		},
		ProviderHash:       reviewsProviderHash,
		Schemas:            map[string]model.Schemas{graphqlmodel.Namespace: providerSchemas},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	return consumer, provider
}

func reviewsBreaksCheckedFrom(checked model.PersistedResource, counterparts model.ResourceCounterparts, counterpartName string) []reviewsBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(reviewsWithoutVerdict{}, []compatibility_checker.ProtocolHooks{graphqlcontracts.New()}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(reviewsEnvironment),
		counterparts,
	)

	var breaks []reviewsBreak
	for _, found := range report.Results[counterpartName].Breaks {
		breaks = append(breaks, reviewsBreak{found.Reason, found.Details})
	}

	return breaks
}

func assertReviewsBreaks(t *testing.T, consumerSchemas, providerSchemas model.Schemas, expected []reviewsBreak) {
	t.Helper()

	consumer, provider := reviewsResources(consumerSchemas, providerSchemas)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected, reviewsBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{reviewsProviderHash: provider},
			RegisteredProviders: map[string]model.RegisteredProvider{reviewsProviderName: {DeployedVersion: null.StringFrom("1.0.0")}},
		}, reviewsProviderName))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected, reviewsBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{reviewsProviderHash: {consumer}},
		}, reviewsConsumerName))
	})
}

func reviewsDetails(argument, propertyType string, fields map[string]string) map[string]string {
	details := map[string]string{
		"property":     "$.reviews",
		"argument":     argument,
		"consumerName": reviewsConsumerName,
		"providerName": reviewsProviderName,
		"propertyType": propertyType,
	}

	for side, field := range fields {
		details[side+"Field"] = field
	}

	return details
}

func reviewsPet(arguments string) model.Schema {
	return model.Schema{
		"":          {Type: "object"},
		"reviews":   {Type: "array", Arguments: arguments},
		"reviews[]": {Type: "string"},
	}
}

func TestNestedArguments_AnArgumentTheProviderRemoved_BreaksOnTheField(t *testing.T) {
	consumer := model.Schemas{
		"Pet":         reviewsPet("ReviewsArgs"),
		"ReviewsArgs": {"": {Type: "object"}, "first": {Type: "integer"}},
	}
	provider := model.Schemas{
		"Query:pet:returns":     reviewsPet("Pet:reviews:arguments"),
		"Pet:reviews:arguments": {"": {Type: "object"}, "after": {Type: "string", Optional: true, Nullable: true}},
	}

	assertReviewsBreaks(t, consumer, provider, []reviewsBreak{{
		model.ReasonPropertyMissingInProvider,
		reviewsDetails("first", "Int!", map[string]string{"consumer": "ReviewsArgs.first"}),
	}})
}

func TestNestedArguments_AFieldWhoseArgumentsTheProviderDropped_BreaksOnTheField(t *testing.T) {
	consumer := model.Schemas{
		"Pet":         reviewsPet("ReviewsArgs"),
		"ReviewsArgs": {"": {Type: "object"}, "first": {Type: "integer"}},
	}
	provider := model.Schemas{"Query:pet:returns": reviewsPet("")}

	assertReviewsBreaks(t, consumer, provider, []reviewsBreak{{
		model.ReasonPropertyMissingInProvider,
		reviewsDetails("first", "Int!", map[string]string{"consumer": "ReviewsArgs.first"}),
	}})
}

func TestNestedArguments_ANewRequiredArgument_BreaksTheConsumerWithoutArguments(t *testing.T) {
	consumer := model.Schemas{"Pet": reviewsPet("")}
	provider := model.Schemas{
		"Query:pet:returns":     reviewsPet("Pet:reviews:arguments"),
		"Pet:reviews:arguments": {"": {Type: "object"}, "first": {Type: "integer"}},
	}

	assertReviewsBreaks(t, consumer, provider, []reviewsBreak{{
		model.ReasonPropertyMissingInConsumer,
		reviewsDetails("first", "Int!", map[string]string{"provider": "Pet.reviews(first:)"}),
	}})
}

func TestNestedArguments_AnInputFieldTheProviderRemoved_NamesItsPathInsideTheArguments(t *testing.T) {
	consumer := model.Schemas{
		"Pet":          reviewsPet("ReviewsArgs"),
		"ReviewsArgs":  {"": {Type: "object"}, "filter": {Ref: "ReviewFilter"}},
		"ReviewFilter": {"": {Type: "object"}, "minStars": {Type: "integer"}},
	}
	provider := model.Schemas{
		"Query:pet:returns":     reviewsPet("Pet:reviews:arguments"),
		"Pet:reviews:arguments": {"": {Type: "object"}, "filter": {Ref: "ReviewFilter"}},
		"ReviewFilter":          {"": {Type: "object"}},
	}

	assertReviewsBreaks(t, consumer, provider, []reviewsBreak{{
		model.ReasonPropertyMissingInProvider,
		reviewsDetails("filter.minStars", "Int!", map[string]string{"consumer": "ReviewFilter.minStars"}),
	}})
}

func TestNestedArguments_TheSameArguments_AreCompatible(t *testing.T) {
	consumer := model.Schemas{
		"Pet":         reviewsPet("ReviewsArgs"),
		"ReviewsArgs": {"": {Type: "object"}, "first": {Type: "integer"}},
	}
	provider := model.Schemas{
		"Query:pet:returns":     reviewsPet("Pet:reviews:arguments"),
		"Pet:reviews:arguments": {"": {Type: "object"}, "first": {Type: "integer", Optional: true, Nullable: true}},
	}

	assertReviewsBreaks(t, consumer, provider, nil)
}

func TestNestedArguments_NoArgumentsOnEitherSide_AreCompatible(t *testing.T) {
	assertReviewsBreaks(t, model.Schemas{"Pet": reviewsPet("")}, model.Schemas{"Query:pet:returns": reviewsPet("")}, nil)
}
