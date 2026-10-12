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
	avatarsConsumerName = "spa"
	avatarsProviderName = "api"
	avatarsEnvironment  = "production"
	avatarsProviderHash = "put-api-profiles"
	avatarsEndpoint     = "/profiles"
	avatarsMethod       = "put"
)

type avatarsWithoutVerdict struct{}

func (this avatarsWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type avatarsBreak struct {
	reason  compatibility_checker.BreakingReason
	role    compatibility_checker.BreakRole
	details map[string]string
}

func avatarsSchema(fields model.Schema) model.Schema {
	schema := model.Schema{"": {Type: "object"}}
	for path, field := range fields {
		schema[path] = field
	}

	return schema
}

func avatarsResources(
	interaction string,
	consumerSchema, providerSchema model.Schema,
) (consumer, provider model.PersistedResource) {
	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumer = model.PersistedResource{
		ParticipantName:    avatarsConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(avatarsProviderName),
		ConsumerHash:       null.StringFrom("spa-" + avatarsProviderHash),
		ProviderHash:       avatarsProviderHash,
		Endpoint:           avatarsEndpoint,
		Method:             avatarsMethod,
		ResponseStatusCode: status,
		Bodies:             []model.ResourceBody{{Source: "spa.yaml", SchemaName: null.StringFrom("Profile")}},
		Schemas:            model.Schemas{"Profile": consumerSchema},
	}

	provider = model.PersistedResource{
		ParticipantName:    avatarsProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       avatarsProviderHash,
		Endpoint:           avatarsEndpoint,
		Method:             avatarsMethod,
		ResponseStatusCode: status,
		Bodies:             []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Profile")}},
		Schemas:            model.Schemas{"Profile": providerSchema},
		DeployedVersions:   map[string]string{avatarsEnvironment: "1.0.0"},
	}

	return consumer, provider
}

func avatarsBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []avatarsBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(avatarsWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(avatarsEnvironment),
		counterparts,
	)

	var breaks []avatarsBreak
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints[avatarsEndpoint][avatarsMethod][interaction] {
		breaks = append(breaks, avatarsBreak{leaf.Reason, leaf.Role, leaf.Details})
	}

	return breaks
}

func avatarsBreakAt(
	reason compatibility_checker.BreakingReason,
	role compatibility_checker.BreakRole,
	property, propertyType string,
) avatarsBreak {
	return avatarsBreak{reason, role, map[string]string{
		"property":     property,
		"consumerName": avatarsConsumerName,
		"providerName": avatarsProviderName,
		"propertyType": propertyType,
	}}
}

type avatarsExpectation func(role compatibility_checker.BreakRole) []avatarsBreak

func noAvatarsBreak(compatibility_checker.BreakRole) []avatarsBreak {
	return nil
}

func assertAvatarsBreaks(
	t *testing.T,
	interaction string,
	consumerSchema, providerSchema model.Schema,
	expected avatarsExpectation,
) {
	t.Helper()

	consumer, provider := avatarsResources(interaction, consumerSchema, providerSchema)

	t.Run("checked from the consumer", func(t *testing.T) {
		assert.Equal(t, expected(compatibility_checker.RoleConsumer), avatarsBreaksCheckedFrom(consumer, model.ResourceCounterparts{
			Providers: map[string]model.PersistedResource{avatarsProviderHash: provider},
		}, avatarsProviderName, interaction))
	})

	t.Run("checked from the provider", func(t *testing.T) {
		assert.Equal(t, expected(compatibility_checker.RoleProvider), avatarsBreaksCheckedFrom(provider, model.ResourceCounterparts{
			Consumers: map[string][]model.PersistedResource{avatarsProviderHash: {consumer}},
		}, avatarsConsumerName, interaction))
	})
}

func avatarProperty(nullable bool) model.Schema {
	return avatarsSchema(model.Schema{"avatar": {Type: "string", Nullable: nullable}})
}

func TestNullableResponse_ProviderNullableConsumerNot_Breaks(t *testing.T) {
	assertAvatarsBreaks(t, "200", avatarProperty(false), avatarProperty(true),
		func(role compatibility_checker.BreakRole) []avatarsBreak {
			return []avatarsBreak{avatarsBreakAt(
				compatibility_checker.ReasonPropertyNullableInProviderNotNullableInConsumer, role, "$.avatar", "string")}
		})
}

func TestNullableResponse_ConsumerNullableProviderNot_IsCompatible(t *testing.T) {
	assertAvatarsBreaks(t, "200", avatarProperty(true), avatarProperty(false), noAvatarsBreak)
}

func TestNullableResponse_BothNullable_IsCompatible(t *testing.T) {
	assertAvatarsBreaks(t, "200", avatarProperty(true), avatarProperty(true), noAvatarsBreak)
}

func TestNullableRequest_ConsumerNullableProviderNot_Breaks(t *testing.T) {
	assertAvatarsBreaks(t, "request", avatarProperty(true), avatarProperty(false),
		func(role compatibility_checker.BreakRole) []avatarsBreak {
			return []avatarsBreak{avatarsBreakAt(
				compatibility_checker.ReasonPropertyNullableInConsumerNotNullableInProvider, role, "$.avatar", "string")}
		})
}

func TestNullableRequest_ProviderNullableConsumerNot_IsCompatible(t *testing.T) {
	assertAvatarsBreaks(t, "request", avatarProperty(false), avatarProperty(true), noAvatarsBreak)
}

func TestNullableRequest_BothNullable_IsCompatible(t *testing.T) {
	assertAvatarsBreaks(t, "request", avatarProperty(true), avatarProperty(true), noAvatarsBreak)
}

func TestNullable_PathMissingOnOneSide_IsNotANullabilityBreak(t *testing.T) {
	optionalAvatar := avatarsSchema(model.Schema{"avatar": {Type: "string", Optional: true, Nullable: true}})

	t.Run("response", func(t *testing.T) {
		assertAvatarsBreaks(t, "200", optionalAvatar, avatarsSchema(nil), noAvatarsBreak)
	})

	t.Run("request", func(t *testing.T) {
		assertAvatarsBreaks(t, "request", avatarsSchema(nil), optionalAvatar, noAvatarsBreak)
	})
}

func TestNullable_TypeNotAccepted_ReportsOnlyTheTypeMismatch(t *testing.T) {
	stringAvatar := avatarsSchema(model.Schema{"avatar": {Type: "string", Nullable: true}})
	integerAvatar := avatarsSchema(model.Schema{"avatar": {Type: "integer"}})

	typeMismatch := func(role compatibility_checker.BreakRole) []avatarsBreak {
		return []avatarsBreak{{compatibility_checker.ReasonPropertyTypeMismatch, role, map[string]string{
			"property":             "$.avatar",
			"consumerName":         avatarsConsumerName,
			"providerName":         avatarsProviderName,
			"consumerPropertyType": "integer",
			"providerPropertyType": "string",
		}}}
	}

	t.Run("response", func(t *testing.T) {
		assertAvatarsBreaks(t, "200", integerAvatar, stringAvatar, typeMismatch)
	})

	t.Run("request", func(t *testing.T) {
		assertAvatarsBreaks(t, "request", integerAvatar, stringAvatar, typeMismatch)
	})
}

func TestNullableResponse_IsIndependentFromOptional(t *testing.T) {
	required := avatarsSchema(model.Schema{"avatar": {Type: "string"}})
	optionalAndNullable := avatarsSchema(model.Schema{"avatar": {Type: "string", Optional: true, Nullable: true}})

	assertAvatarsBreaks(t, "200", required, optionalAndNullable,
		func(role compatibility_checker.BreakRole) []avatarsBreak {
			return []avatarsBreak{
				avatarsBreakAt(compatibility_checker.ReasonPropertyNullableInProviderNotNullableInConsumer, role, "$.avatar", "string"),
				avatarsBreakAt(compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, role, "$.avatar", "string"),
			}
		})
}

func TestNullableResponse_ChildrenOfANullableParent_AreComparedNormally(t *testing.T) {
	consumerManager := avatarsSchema(model.Schema{
		"manager":       {Type: "object", Nullable: true},
		"manager.name":  {Type: "string"},
		"manager.email": {Type: "string"},
	})
	providerManager := avatarsSchema(model.Schema{
		"manager":      {Type: "object", Nullable: true},
		"manager.name": {Type: "string"},
	})

	assertAvatarsBreaks(t, "200", consumerManager, providerManager,
		func(role compatibility_checker.BreakRole) []avatarsBreak {
			return []avatarsBreak{avatarsBreakAt(
				compatibility_checker.ReasonPropertyMissingInProvider, role, "$.manager.email", "string")}
		})
}

func TestNullableRequest_ChildrenOfANullableParent_AreComparedNormally(t *testing.T) {
	consumerManager := avatarsSchema(model.Schema{
		"manager":      {Type: "object", Nullable: true},
		"manager.name": {Type: "string"},
	})
	providerManager := avatarsSchema(model.Schema{
		"manager":       {Type: "object", Nullable: true},
		"manager.name":  {Type: "string"},
		"manager.email": {Type: "string"},
	})

	assertAvatarsBreaks(t, "request", consumerManager, providerManager,
		func(role compatibility_checker.BreakRole) []avatarsBreak {
			return []avatarsBreak{avatarsBreakAt(
				compatibility_checker.ReasonPropertyMissingInConsumer, role, "$.manager.email", "string")}
		})
}
