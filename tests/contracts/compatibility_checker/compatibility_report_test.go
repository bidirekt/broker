package compatibility_checker_test

import (
	"context"
	"maps"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	petstoreWeb    = "petstore_web"
	petstoreMobile = "petstore_mobile"
	petstoreAPI    = "petstore_api"
	production     = "production"
	postPetsHash   = "post-pets"
)

type noStoredVerdicts struct{}

func (this noStoredVerdicts) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

func postPetsResources(
	interaction string,
	consumerSchema, providerSchema model.Schema,
) (consumer, provider model.PersistedResource) {
	body, status := model.Output, interaction
	if interaction == "request" {
		body, status = model.Input, ""
	}

	consumer = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  petstoreWeb,
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             body,
			Identity:         restmodel.Identity{Endpoint: "/pets", Method: "post", Status: status},
			ConsumedProvider: null.StringFrom(petstoreAPI),
			Bodies:           []model.ResourceBody{{Source: "web.yaml", SchemaName: null.StringFrom("Pet")}},
		},
		ConsumerHash: null.StringFrom("web-" + postPetsHash),
		ProviderHash: postPetsHash,
		Schemas:      model.Schemas{"Pet": consumerSchema},
	}

	provider = model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: petstoreAPI,
			Direction:       model.Provides,
			Protocol:        restmodel.Protocol,
			Body:            body,
			Identity:        restmodel.Identity{Endpoint: "/pets", Method: "post", Status: status},
			Bodies:          []model.ResourceBody{{Source: "api.yaml", SchemaName: null.StringFrom("Pet")}},
		},
		ProviderHash:       postPetsHash,
		Schemas:            model.Schemas{"Pet": providerSchema},
		ParticipantVersion: null.StringFrom("1.0.0"),
	}

	return consumer, provider
}

func checkFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
) *compatibility_checker.ContractCompatibilityReport {
	contract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	return compatibility_checker.NewCompatibilityChecker(noStoredVerdicts{}, []compatibility_checker.ProtocolHooks{restcontracts.New()}).Check(
		context.Background(),
		contract,
		model.NewEnvironment(production),
		counterparts,
	)
}

func TestFreshPropertyBreaksCarryTheRoleOfTheCheckedSide(t *testing.T) {
	required := model.Schema{"": {Type: "object"}, "name": {Type: "string"}}
	optional := model.Schema{"": {Type: "object"}, "name": {Type: "string", Optional: true}}
	integer := model.Schema{"": {Type: "object"}, "name": {Type: "integer"}}
	absent := model.Schema{"": {Type: "object"}}

	cases := []struct {
		reason      model.BreakingReason
		interaction string
		consumer    model.Schema
		provider    model.Schema
	}{
		{model.ReasonPropertyMissingInProvider, "201", required, absent},
		{model.ReasonPropertyOptionalInProviderRequiredInConsumer, "201", required, optional},
		{model.ReasonPropertyTypeMismatch, "201", required, integer},
		{model.ReasonPropertyMissingInConsumer, "request", absent, required},
		{model.ReasonPropertyOptionalInConsumerRequiredInProvider, "request", optional, required},
		{model.ReasonPropertyTypeMismatch, "request", integer, required},
	}

	for _, tc := range cases {
		consumer, provider := postPetsResources(tc.interaction, tc.consumer, tc.provider)

		t.Run(string(tc.reason)+" in "+tc.interaction+" checked from the consumer", func(t *testing.T) {
			report := checkFrom(consumer, model.ResourceCounterparts{
				Providers:           map[string]model.PersistedResource{postPetsHash: provider},
				RegisteredProviders: map[string]model.RegisteredProvider{petstoreAPI: {DeployedVersion: null.StringFrom("1.0.0")}},
			})

			leaves := report.Hierarchical[petstoreAPI].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"][tc.interaction]
			require.Len(t, leaves, 1)
			assert.Equal(t, tc.reason, leaves[0].Reason)
			assert.Equal(t, model.RoleConsumer, leaves[0].Role)
		})

		t.Run(string(tc.reason)+" in "+tc.interaction+" checked from the provider", func(t *testing.T) {
			report := checkFrom(provider, model.ResourceCounterparts{
				Consumers: map[string][]model.PersistedResource{postPetsHash: {consumer}},
			})

			leaves := report.Hierarchical[petstoreWeb].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"][tc.interaction]
			require.Len(t, leaves, 1)
			assert.Equal(t, tc.reason, leaves[0].Reason)
			assert.Equal(t, model.RoleProvider, leaves[0].Role)
		})
	}
}

func TestFreshResourceBreaksCarryTheirFixedRole(t *testing.T) {
	pet := model.Schema{"": {Type: "object"}, "name": {Type: "string"}}
	consumer, provider := postPetsResources("201", pet, pet)

	removed := provider
	removed.Removed = true

	cases := []struct {
		reason       model.BreakingReason
		checked      model.PersistedResource
		counterparts model.ResourceCounterparts
		counterpart  string
		role         model.BreakRole
	}{
		{
			reason:  model.ReasonProviderResourceNotFound,
			checked: consumer,
			counterparts: model.ResourceCounterparts{
				RegisteredProviders: map[string]model.RegisteredProvider{petstoreAPI: {DeployedVersion: null.StringFrom("1.0.0")}},
			},
			counterpart: petstoreAPI,
			role:        model.RoleConsumer,
		},
		{
			reason:       model.ReasonProviderResourceNotDeployedInEnvironment,
			checked:      consumer,
			counterparts: model.ResourceCounterparts{},
			counterpart:  petstoreAPI,
			role:         model.RoleConsumer,
		},
		{
			reason:  model.ReasonProviderResourceRemovedButStillConsumed,
			checked: removed,
			counterparts: model.ResourceCounterparts{
				Consumers: map[string][]model.PersistedResource{postPetsHash: {consumer}},
			},
			counterpart: petstoreWeb,
			role:        model.RoleProvider,
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.reason), func(t *testing.T) {
			report := checkFrom(tc.checked, tc.counterparts)

			leaves := report.Hierarchical[tc.counterpart].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"]["201"]
			require.Len(t, leaves, 1)
			assert.Equal(t, tc.reason, leaves[0].Reason)
			assert.Equal(t, tc.role, leaves[0].Role)
		})
	}
}

func TestCachedPropertyBreaksCarryTheRoleOfTheCurrentCheck(t *testing.T) {
	cases := []struct {
		reason      model.BreakingReason
		interaction string
		types       map[string]string
	}{
		{model.ReasonPropertyMissingInProvider, "201", map[string]string{"propertyType": "string"}},
		{model.ReasonPropertyOptionalInProviderRequiredInConsumer, "201", map[string]string{"propertyType": "string"}},
		{model.ReasonPropertyTypeMismatch, "201", map[string]string{"consumerPropertyType": "string", "providerPropertyType": "integer"}},
		{model.ReasonPropertyMissingInConsumer, "request", map[string]string{"propertyType": "string"}},
		{model.ReasonPropertyOptionalInConsumerRequiredInProvider, "request", map[string]string{"propertyType": "string"}},
		{model.ReasonPropertyTypeMismatch, "request", map[string]string{"consumerPropertyType": "integer", "providerPropertyType": "string"}},
	}

	for _, tc := range cases {
		details := map[string]string{"property": "$.name", "consumerName": petstoreWeb, "providerName": petstoreAPI}
		maps.Copy(details, tc.types)

		stored := model.VerdictBreak{
			Protocol: restmodel.Protocol,
			Location: map[string]string{restmodel.EndpointKey: "/pets", restmodel.MethodKey: "post", restmodel.InteractionKey: tc.interaction},
			Reason:   tc.reason,
			Details:  details,
		}

		t.Run(string(tc.reason)+" in "+tc.interaction+" replayed to the consumer", func(t *testing.T) {
			item := compatibility_checker.NewCounterpartResult()
			item.AppendCachedBreakChange(stored)

			report := compatibility_checker.NewContractCompatibilityReport(petstoreWeb, "2.0.0", production, []compatibility_checker.ProtocolHooks{restcontracts.New()})
			report.AppendResult(petstoreAPI, item)

			assert.Equal(t, []model.ReportBreak{{
				Reason:  tc.reason,
				Role:    model.RoleConsumer,
				Details: details,
			}}, report.Hierarchical[petstoreAPI].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"][tc.interaction])
		})

		t.Run(string(tc.reason)+" in "+tc.interaction+" replayed to the provider", func(t *testing.T) {
			item := compatibility_checker.NewCounterpartResult()
			item.AppendCachedBreakChange(stored)

			report := compatibility_checker.NewContractCompatibilityReport(petstoreAPI, "4.0.0", production, []compatibility_checker.ProtocolHooks{restcontracts.New()})
			report.AppendResult(petstoreWeb, item)

			assert.Equal(t, []model.ReportBreak{{
				Reason:  tc.reason,
				Role:    model.RoleProvider,
				Details: details,
			}}, report.Hierarchical[petstoreWeb].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"][tc.interaction])
		})
	}
}

func TestFreshBreaksOfAnInteractionAreSortedByProperty(t *testing.T) {
	readByTheConsumer := model.Schema{"": {Type: "object"}}
	for _, property := range []string{"weight", "photoUrl", "ownerId", "name", "age", "color"} {
		readByTheConsumer[property] = model.Field{Type: "string"}
	}

	consumer, provider := postPetsResources("201", readByTheConsumer, model.Schema{"": {Type: "object"}})

	report := checkFrom(consumer, model.ResourceCounterparts{
		Providers:           map[string]model.PersistedResource{postPetsHash: provider},
		RegisteredProviders: map[string]model.RegisteredProvider{petstoreAPI: {DeployedVersion: null.StringFrom("1.0.0")}},
	})

	var properties []string
	for _, leaf := range report.Hierarchical[petstoreAPI].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"]["201"] {
		properties = append(properties, leaf.Details["property"])
	}

	assert.Equal(t, []string{"$.age", "$.color", "$.name", "$.ownerId", "$.photoUrl", "$.weight"}, properties)
}

func TestCachedBreaksOfAnInteractionAreSortedByProperty(t *testing.T) {
	item := compatibility_checker.NewCounterpartResult()
	for _, property := range []string{"$.weight", "$.photoUrl"} {
		item.AppendCachedBreakChange(model.VerdictBreak{
			Protocol: restmodel.Protocol,
			Location: map[string]string{restmodel.EndpointKey: "/pets", restmodel.MethodKey: "post", restmodel.InteractionKey: "201"},
			Reason:   model.ReasonPropertyMissingInProvider,
			Details:  map[string]string{"property": property, "consumerName": petstoreWeb, "providerName": petstoreAPI},
		})
	}

	report := compatibility_checker.NewContractCompatibilityReport(petstoreWeb, "2.0.0", production, []compatibility_checker.ProtocolHooks{restcontracts.New()})
	report.AppendResult(petstoreAPI, item)

	leaves := report.Hierarchical[petstoreAPI].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["post"]["201"]
	require.Len(t, leaves, 2)
	assert.Equal(t, "$.photoUrl", leaves[0].Details["property"])
	assert.Equal(t, "$.weight", leaves[1].Details["property"])
}

func TestReportIsDeployableOnlyWhenEveryResultIs(t *testing.T) {
	t.Run("without results", func(t *testing.T) {
		report := compatibility_checker.NewContractCompatibilityReport(petstoreAPI, "4.0.0", production, []compatibility_checker.ProtocolHooks{restcontracts.New()})

		assert.True(t, report.Deployable())
	})

	t.Run("with every result deployable", func(t *testing.T) {
		report := compatibility_checker.NewContractCompatibilityReport(petstoreAPI, "4.0.0", production, []compatibility_checker.ProtocolHooks{restcontracts.New()})
		report.AppendResult(petstoreWeb, compatibility_checker.NewCounterpartResult())
		report.AppendResult(petstoreMobile, compatibility_checker.NewCounterpartResult())

		assert.True(t, report.Deployable())
	})

	t.Run("with one result not deployable", func(t *testing.T) {
		broken := compatibility_checker.NewCounterpartResult()
		broken.AppendCachedBreakChange(model.VerdictBreak{
			Protocol: restmodel.Protocol,
			Location: map[string]string{restmodel.EndpointKey: "/pets", restmodel.MethodKey: "post", restmodel.InteractionKey: "201"},
			Reason:   model.ReasonPropertyMissingInProvider,
			Details:  map[string]string{"property": "$.name", "consumerName": petstoreMobile, "providerName": petstoreAPI},
		})

		report := compatibility_checker.NewContractCompatibilityReport(petstoreAPI, "4.0.0", production, []compatibility_checker.ProtocolHooks{restcontracts.New()})
		report.AppendResult(petstoreWeb, compatibility_checker.NewCounterpartResult())
		report.AppendResult(petstoreMobile, broken)

		assert.False(t, report.Deployable())
	})
}

func TestVerdictBreaksKeepOnlyPropertyBreaksUnderTheirProtocolLocation(t *testing.T) {
	consumer, provider := postPetsResources("201", model.Schema{"": {Type: "object"}, "name": {Type: "string"}}, model.Schema{"": {Type: "object"}})

	item := compatibility_checker.NewCounterpartResult()
	item.AppendContractBreakChange(compatibility_checker.NewProviderResourceNotFound(&consumer))
	item.AppendContractBreakChange(compatibility_checker.NewContractBreakingChange(&consumer, &provider, model.ReasonPropertyMissingInProvider))

	assert.Equal(t, []model.VerdictBreak{{
		Protocol: restmodel.Protocol,
		Location: map[string]string{restmodel.EndpointKey: "/pets", restmodel.MethodKey: "post", restmodel.InteractionKey: "201"},
		Reason:   model.ReasonPropertyMissingInProvider,
	}}, compatibility_checker.ToVerdictBreaks(*item))
}
