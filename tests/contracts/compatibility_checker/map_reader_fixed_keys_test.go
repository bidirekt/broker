package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	onboardingConsumerName = "spa"
	onboardingProviderName = "api"
	onboardingEnvironment  = "production"
	onboardingProviderHash = "put-api-session"
	onboardingEndpoint     = "/api/session"
	onboardingMethod       = "put"
)

type onboardingWithoutVerdict struct{}

func (this onboardingWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type onboardingBreak struct {
	reason  compatibility_checker.BreakingReason
	details map[string]string
}

type onboardingSide struct {
	bodies  []model.ResourceBody
	schemas model.Schemas
}

func onboardingBody(schemaName string, schema model.Schema) onboardingSide {
	return onboardingSide{
		bodies:  []model.ResourceBody{{Source: "session.yaml", SchemaName: null.StringFrom(schemaName)}},
		schemas: model.Schemas{schemaName: schema},
	}
}

func coreSession(schema model.Schema) onboardingSide {
	return onboardingBody("CoreSession", schema)
}

func onboardingValues(values model.Field) model.Schema {
	return model.Schema{
		"":             {Type: "object"},
		"onboarding":   {Type: "object"},
		"onboarding{}": values,
	}
}

func onboardingKeys(profile, church model.Field) model.Schema {
	return model.Schema{
		"":                   {Type: "object"},
		"onboarding":         {Type: "object"},
		"onboarding.profile": profile,
		"onboarding.church":  church,
	}
}

func onboardingBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []onboardingBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(onboardingWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(onboardingEnvironment),
		counterparts,
	)

	var breaks []onboardingBreak
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints[onboardingEndpoint][onboardingMethod][interaction] {
		breaks = append(breaks, onboardingBreak{leaf.Reason, leaf.Details})
	}

	return breaks
}

func onboardingBreaks(t *testing.T, interaction string, consumerSide, providerSide onboardingSide) []onboardingBreak {
	t.Helper()

	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumer := model.PersistedResource{
		ParticipantName:    onboardingConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(onboardingProviderName),
		ConsumerHash:       null.StringFrom("spa-" + onboardingProviderHash),
		ProviderHash:       onboardingProviderHash,
		Endpoint:           onboardingEndpoint,
		Method:             onboardingMethod,
		ResponseStatusCode: status,
		Bodies:             consumerSide.bodies,
		Schemas:            consumerSide.schemas,
	}
	provider := model.PersistedResource{
		ParticipantName:    onboardingProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       onboardingProviderHash,
		Endpoint:           onboardingEndpoint,
		Method:             onboardingMethod,
		ResponseStatusCode: status,
		Bodies:             providerSide.bodies,
		Schemas:            providerSide.schemas,
		DeployedVersions:   map[string]string{onboardingEnvironment: "1.0.0"},
	}

	fromTheConsumer := onboardingBreaksCheckedFrom(consumer, model.ResourceCounterparts{
		Providers: map[string]model.PersistedResource{onboardingProviderHash: provider},
	}, onboardingProviderName, interaction)

	fromTheProvider := onboardingBreaksCheckedFrom(provider, model.ResourceCounterparts{
		Consumers: map[string][]model.PersistedResource{onboardingProviderHash: {consumer}},
	}, onboardingConsumerName, interaction)

	require.Equal(t, fromTheConsumer, fromTheProvider, "the same pair checked from either side")

	return fromTheConsumer
}

func onboardingDetails(property string, specific map[string]string) map[string]string {
	details := map[string]string{"property": property, "consumerName": onboardingConsumerName, "providerName": onboardingProviderName}
	for key, value := range specific {
		details[key] = value
	}

	return details
}

func TestMapReader_FixedKeysOfTheValueType_HaveNoBreak(t *testing.T) {
	readAsAMap := onboardingValues(model.Field{Type: "boolean"})
	fixedKeys := onboardingKeys(model.Field{Type: "boolean"}, model.Field{Type: "boolean"})

	t.Run("in a response", func(t *testing.T) {
		assert.Empty(t, onboardingBreaks(t, "200", coreSession(readAsAMap), coreSession(fixedKeys)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Empty(t, onboardingBreaks(t, "request", coreSession(fixedKeys), coreSession(readAsAMap)))
	})
}

func TestMapReader_FixedKeyOfAnotherType_BreaksOnItsPath(t *testing.T) {
	readAsAMap := onboardingValues(model.Field{Type: "boolean"})
	fixedKeys := onboardingKeys(model.Field{Type: "boolean"}, model.Field{Type: "string"})

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.onboarding.church", map[string]string{
				"consumerField": "CoreSession.onboarding{}", "providerField": "CoreSession.onboarding.church",
				"consumerPropertyType": "boolean", "providerPropertyType": "string",
			})},
		}, onboardingBreaks(t, "200", coreSession(readAsAMap), coreSession(fixedKeys)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.onboarding.church", map[string]string{
				"consumerField": "CoreSession.onboarding.church", "providerField": "CoreSession.onboarding{}",
				"consumerPropertyType": "string", "providerPropertyType": "boolean",
			})},
		}, onboardingBreaks(t, "request", coreSession(fixedKeys), coreSession(readAsAMap)))
	})
}

func TestMapReader_NullableFixedKey_BreaksOnlyAgainstValuesThatAreNotNullable(t *testing.T) {
	nullableChurch := onboardingKeys(model.Field{Type: "boolean"}, model.Field{Type: "boolean", Nullable: true})

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyNullableInProviderNotNullableInConsumer, onboardingDetails("$.onboarding.church", map[string]string{
				"consumerField": "CoreSession.onboarding{}", "providerField": "CoreSession.onboarding.church", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "200", coreSession(onboardingValues(model.Field{Type: "boolean"})), coreSession(nullableChurch)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyNullableInConsumerNotNullableInProvider, onboardingDetails("$.onboarding.church", map[string]string{
				"consumerField": "CoreSession.onboarding.church", "providerField": "CoreSession.onboarding{}", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "request", coreSession(nullableChurch), coreSession(onboardingValues(model.Field{Type: "boolean"}))))
	})

	t.Run("against nullable values", func(t *testing.T) {
		nullableValues := onboardingValues(model.Field{Type: "boolean", Nullable: true})

		assert.Empty(t, onboardingBreaks(t, "200", coreSession(nullableValues), coreSession(nullableChurch)))
		assert.Empty(t, onboardingBreaks(t, "request", coreSession(nullableChurch), coreSession(nullableValues)))
	})
}

func TestMapReader_OptionalFixedKey_NeverBreaks(t *testing.T) {
	readAsAMap := onboardingValues(model.Field{Type: "boolean"})
	optionalProfile := onboardingKeys(model.Field{Type: "boolean", Optional: true}, model.Field{Type: "boolean"})

	t.Run("provided only sometimes in a response", func(t *testing.T) {
		assert.Empty(t, onboardingBreaks(t, "200", coreSession(readAsAMap), coreSession(optionalProfile)))
	})

	t.Run("sent only sometimes in a request", func(t *testing.T) {
		assert.Empty(t, onboardingBreaks(t, "request", coreSession(optionalProfile), coreSession(readAsAMap)))
	})

	t.Run("sent by only some consumer modules in a request", func(t *testing.T) {
		twoModules := onboardingSide{
			bodies: []model.ResourceBody{
				{Source: "profile.yaml", SchemaName: null.StringFrom("ProfileStep")},
				{Source: "church.yaml", SchemaName: null.StringFrom("ChurchStep")},
			},
			schemas: model.Schemas{
				"ProfileStep": {
					"":                   {Type: "object"},
					"onboarding":         {Type: "object"},
					"onboarding.profile": {Type: "boolean"},
				},
				"ChurchStep": onboardingKeys(model.Field{Type: "boolean"}, model.Field{Type: "boolean"}),
			},
		}

		assert.Empty(t, onboardingBreaks(t, "request", twoModules, coreSession(readAsAMap)))
	})
}

func TestMapReader_FixedKeyContent_FollowsEveryRuleOfTheValueContent(t *testing.T) {
	readAsAMap := model.Schema{
		"":                  {Type: "object"},
		"onboarding":        {Type: "object"},
		"onboarding{}":      {Type: "object"},
		"onboarding{}.done": {Type: "boolean"},
	}
	fixedKeys := model.Schema{
		"":                        {Type: "object"},
		"onboarding":              {Type: "object"},
		"onboarding.profile":      {Type: "object"},
		"onboarding.profile.done": {Type: "boolean", Optional: true},
		"onboarding.church":       {Type: "object"},
	}

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyMissingInProvider, onboardingDetails("$.onboarding.church.done", map[string]string{
				"consumerField": "CoreSession.onboarding{}.done", "propertyType": "boolean",
			})},
			{compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, onboardingDetails("$.onboarding.profile.done", map[string]string{
				"consumerField": "CoreSession.onboarding{}.done", "providerField": "CoreSession.onboarding.profile.done", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "200", coreSession(readAsAMap), coreSession(fixedKeys)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyMissingInConsumer, onboardingDetails("$.onboarding.church.done", map[string]string{
				"providerField": "CoreSession.onboarding{}.done", "propertyType": "boolean",
			})},
			{compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, onboardingDetails("$.onboarding.profile.done", map[string]string{
				"consumerField": "CoreSession.onboarding.profile.done", "providerField": "CoreSession.onboarding{}.done", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "request", coreSession(fixedKeys), coreSession(readAsAMap)))
	})
}

func TestMapReader_ConsumerModulesReadingTheMapAndAFixedKey_CompareEachProvidedKeyOnce(t *testing.T) {
	twoModules := onboardingSide{
		bodies: []model.ResourceBody{
			{Source: "map.yaml", SchemaName: null.StringFrom("SessionMap")},
			{Source: "church.yaml", SchemaName: null.StringFrom("SessionChurch")},
		},
		schemas: model.Schemas{
			"SessionMap": onboardingValues(model.Field{Type: "boolean"}),
			"SessionChurch": {
				"":                  {Type: "object"},
				"onboarding":        {Type: "object"},
				"onboarding.church": {Type: "string"},
			},
		},
	}

	t.Run("a key also read as a fixed key is compared only against it", func(t *testing.T) {
		fixedKeys := onboardingKeys(model.Field{Type: "boolean"}, model.Field{Type: "string"})

		assert.Empty(t, onboardingBreaks(t, "200", twoModules, coreSession(fixedKeys)))
	})

	t.Run("every other key is compared against the map", func(t *testing.T) {
		fixedKeys := onboardingKeys(model.Field{Type: "integer"}, model.Field{Type: "integer"})

		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.onboarding.church", map[string]string{
				"consumerField": "SessionChurch.onboarding.church", "providerField": "CoreSession.onboarding.church",
				"consumerPropertyType": "string", "providerPropertyType": "integer",
			})},
			{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.onboarding.profile", map[string]string{
				"consumerField": "SessionMap.onboarding{}", "providerField": "CoreSession.onboarding.profile",
				"consumerPropertyType": "boolean", "providerPropertyType": "integer",
			})},
		}, onboardingBreaks(t, "200", twoModules, coreSession(fixedKeys)))
	})
}

func TestMapReader_ConsumerModulesSendingTheMapAndAFixedKey_CompareTheSentKeyAgainstTheMap(t *testing.T) {
	twoModules := onboardingSide{
		bodies: []model.ResourceBody{
			{Source: "map.yaml", SchemaName: null.StringFrom("SessionMap")},
			{Source: "church.yaml", SchemaName: null.StringFrom("SessionChurch")},
		},
		schemas: model.Schemas{
			"SessionMap": onboardingValues(model.Field{Type: "boolean"}),
			"SessionChurch": {
				"":                  {Type: "object"},
				"onboarding":        {Type: "object"},
				"onboarding.church": {Type: "string"},
			},
		},
	}

	assert.Equal(t, []onboardingBreak{
		{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.onboarding.church", map[string]string{
			"consumerField": "SessionChurch.onboarding.church", "providerField": "CoreSession.onboarding{}",
			"consumerPropertyType": "string", "providerPropertyType": "boolean",
		})},
		{compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, onboardingDetails("$.onboarding{}", map[string]string{
			"consumerField": "SessionMap.onboarding{}", "providerField": "CoreSession.onboarding{}", "propertyType": "boolean",
		})},
	}, onboardingBreaks(t, "request", twoModules, coreSession(onboardingValues(model.Field{Type: "boolean"}))))
}

func TestMapReader_ObjectWithoutProperties_IsStillMissingTheValues(t *testing.T) {
	readAsAMap := onboardingValues(model.Field{Type: "boolean"})
	anyObject := model.Schema{
		"":           {Type: "object"},
		"onboarding": {Type: "object"},
	}

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyMissingInProvider, onboardingDetails("$.onboarding{}", map[string]string{
				"consumerField": "CoreSession.onboarding{}", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "200", coreSession(readAsAMap), coreSession(anyObject)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyMissingInConsumer, onboardingDetails("$.onboarding{}", map[string]string{
				"providerField": "CoreSession.onboarding{}", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "request", coreSession(anyObject), coreSession(readAsAMap)))
	})
}

func TestFixedKeyReader_AgainstAMap_IsStillMissingEachKey(t *testing.T) {
	sentAsAMap := onboardingValues(model.Field{Type: "boolean"})
	readAsFixedKeys := onboardingKeys(model.Field{Type: "boolean"}, model.Field{Type: "boolean"})

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyMissingInProvider, onboardingDetails("$.onboarding.church", map[string]string{
				"consumerField": "CoreSession.onboarding.church", "propertyType": "boolean",
			})},
			{compatibility_checker.ReasonPropertyMissingInProvider, onboardingDetails("$.onboarding.profile", map[string]string{
				"consumerField": "CoreSession.onboarding.profile", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "200", coreSession(readAsFixedKeys), coreSession(sentAsAMap)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyMissingInConsumer, onboardingDetails("$.onboarding.church", map[string]string{
				"providerField": "CoreSession.onboarding.church", "propertyType": "boolean",
			})},
			{compatibility_checker.ReasonPropertyMissingInConsumer, onboardingDetails("$.onboarding.profile", map[string]string{
				"providerField": "CoreSession.onboarding.profile", "propertyType": "boolean",
			})},
		}, onboardingBreaks(t, "request", coreSession(sentAsAMap), coreSession(readAsFixedKeys)))
	})
}

func TestMapReader_AtTheBodyRoot_ReadsTheFixedKeysAsValues(t *testing.T) {
	readAsAMap := model.Schema{
		"":   {Type: "object"},
		"{}": {Type: "boolean"},
	}
	fixedKeys := model.Schema{
		"":        {Type: "object"},
		"profile": {Type: "boolean"},
		"church":  {Type: "string"},
	}

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.church", map[string]string{
				"consumerField": "OnboardingSteps{}", "providerField": "OnboardingSteps.church",
				"consumerPropertyType": "boolean", "providerPropertyType": "string",
			})},
		}, onboardingBreaks(t, "200", onboardingBody("OnboardingSteps", readAsAMap), onboardingBody("OnboardingSteps", fixedKeys)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []onboardingBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, onboardingDetails("$.church", map[string]string{
				"consumerField": "OnboardingSteps.church", "providerField": "OnboardingSteps{}",
				"consumerPropertyType": "string", "providerPropertyType": "boolean",
			})},
		}, onboardingBreaks(t, "request", onboardingBody("OnboardingSteps", fixedKeys), onboardingBody("OnboardingSteps", readAsAMap)))
	})
}
