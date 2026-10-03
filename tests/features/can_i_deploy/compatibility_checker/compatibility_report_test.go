package compatibility_checker_test

import (
	"context"
	"maps"
	"testing"

	"github.com/bidirekt/broker/internal/features/can_i_deploy/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	petstoreWeb  = "petstore_web"
	petstoreAPI  = "petstore_api"
	production   = "production"
	postPetsHash = "post-pets"
)

type noStoredVerdicts struct{}

func (this noStoredVerdicts) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

func postPetsResources(
	interaction string,
	consumerProperties, providerProperties map[string]model.Property,
) (consumer, provider model.PersistedResource) {
	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumer = model.PersistedResource{
		ParticipantName:    petstoreWeb,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(petstoreAPI),
		ConsumerHash:       null.StringFrom("web-" + postPetsHash),
		ProviderHash:       postPetsHash,
		Endpoint:           "/pets",
		Method:             "post",
		ResponseStatusCode: status,
		Properties:         consumerProperties,
	}

	provider = model.PersistedResource{
		ParticipantName:    petstoreAPI,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       postPetsHash,
		Endpoint:           "/pets",
		Method:             "post",
		ResponseStatusCode: status,
		Properties:         providerProperties,
		DeployedVersions:   map[string]string{production: "1.0.0"},
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

	return compatibility_checker.NewCompatibilityChecker(noStoredVerdicts{}).Check(
		context.Background(),
		contract,
		model.NewEnvironment(production),
		counterparts,
	)
}

func TestFreshPropertyBreaksCarryTheRoleOfTheCheckedSide(t *testing.T) {
	required := map[string]model.Property{"$.name": model.NewProperty("$.name", "string", false)}
	optional := map[string]model.Property{"$.name": model.NewProperty("$.name", "string", true)}
	integer := map[string]model.Property{"$.name": model.NewProperty("$.name", "integer", false)}
	absent := map[string]model.Property{}

	cases := []struct {
		reason      compatibility_checker.BreakingReason
		interaction string
		consumer    map[string]model.Property
		provider    map[string]model.Property
	}{
		{compatibility_checker.ReasonPropertyMissingInProvider, "201", required, absent},
		{compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, "201", required, optional},
		{compatibility_checker.ReasonPropertyTypeMismatch, "201", required, integer},
		{compatibility_checker.ReasonPropertyMissingInConsumer, "request", absent, required},
		{compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, "request", optional, required},
		{compatibility_checker.ReasonPropertyTypeMismatch, "request", integer, required},
	}

	for _, tc := range cases {
		consumer, provider := postPetsResources(tc.interaction, tc.consumer, tc.provider)

		t.Run(string(tc.reason)+" in "+tc.interaction+" checked from the consumer", func(t *testing.T) {
			report := checkFrom(consumer, model.ResourceCounterparts{
				Providers: map[string]model.PersistedResource{postPetsHash: provider},
			})

			leaves := report.Hierarchical[petstoreAPI].Endpoints["/pets"]["post"][tc.interaction]
			require.Len(t, leaves, 1)
			assert.Equal(t, tc.reason, leaves[0].Reason)
			assert.Equal(t, compatibility_checker.RoleConsumer, leaves[0].Role)
		})

		t.Run(string(tc.reason)+" in "+tc.interaction+" checked from the provider", func(t *testing.T) {
			report := checkFrom(provider, model.ResourceCounterparts{
				Consumers: map[string][]model.PersistedResource{postPetsHash: {consumer}},
			})

			leaves := report.Hierarchical[petstoreWeb].Endpoints["/pets"]["post"][tc.interaction]
			require.Len(t, leaves, 1)
			assert.Equal(t, tc.reason, leaves[0].Reason)
			assert.Equal(t, compatibility_checker.RoleProvider, leaves[0].Role)
		})
	}
}

func TestFreshResourceBreaksCarryTheirFixedRole(t *testing.T) {
	properties := map[string]model.Property{"$.name": model.NewProperty("$.name", "string", false)}
	consumer, provider := postPetsResources("201", properties, properties)

	undeployed := provider
	undeployed.DeployedVersions = map[string]string{"staging": "1.0.0"}

	removed := provider
	removed.Removed = true

	cases := []struct {
		reason       compatibility_checker.BreakingReason
		checked      model.PersistedResource
		counterparts model.ResourceCounterparts
		counterpart  string
		role         compatibility_checker.BreakRole
	}{
		{
			reason:       compatibility_checker.ReasonProviderResourceNotFound,
			checked:      consumer,
			counterparts: model.ResourceCounterparts{},
			counterpart:  petstoreAPI,
			role:         compatibility_checker.RoleConsumer,
		},
		{
			reason:  compatibility_checker.ReasonProviderResourceNotDeployedInEnvironment,
			checked: consumer,
			counterparts: model.ResourceCounterparts{
				Providers: map[string]model.PersistedResource{postPetsHash: undeployed},
			},
			counterpart: petstoreAPI,
			role:        compatibility_checker.RoleConsumer,
		},
		{
			reason:  compatibility_checker.ReasonProviderResourceRemovedButStillConsumed,
			checked: removed,
			counterparts: model.ResourceCounterparts{
				Consumers: map[string][]model.PersistedResource{postPetsHash: {consumer}},
			},
			counterpart: petstoreWeb,
			role:        compatibility_checker.RoleProvider,
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.reason), func(t *testing.T) {
			report := checkFrom(tc.checked, tc.counterparts)

			leaves := report.Hierarchical[tc.counterpart].Endpoints["/pets"]["post"]["201"]
			require.Len(t, leaves, 1)
			assert.Equal(t, tc.reason, leaves[0].Reason)
			assert.Equal(t, tc.role, leaves[0].Role)
		})
	}
}

func TestCachedPropertyBreaksCarryTheRoleOfTheCurrentCheck(t *testing.T) {
	cases := []struct {
		reason      compatibility_checker.BreakingReason
		interaction string
		types       map[string]string
	}{
		{compatibility_checker.ReasonPropertyMissingInProvider, "201", map[string]string{"propertyType": "string"}},
		{compatibility_checker.ReasonPropertyOptionalInProviderRequiredInConsumer, "201", map[string]string{"propertyType": "string"}},
		{compatibility_checker.ReasonPropertyTypeMismatch, "201", map[string]string{"consumerPropertyType": "string", "providerPropertyType": "integer"}},
		{compatibility_checker.ReasonPropertyMissingInConsumer, "request", map[string]string{"propertyType": "string"}},
		{compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, "request", map[string]string{"propertyType": "string"}},
		{compatibility_checker.ReasonPropertyTypeMismatch, "request", map[string]string{"consumerPropertyType": "integer", "providerPropertyType": "string"}},
	}

	for _, tc := range cases {
		details := map[string]string{"property": "$.name", "consumerName": petstoreWeb, "providerName": petstoreAPI}
		maps.Copy(details, tc.types)

		stored := model.VerdictBreak{
			Endpoint:    "/pets",
			Method:      "post",
			Interaction: tc.interaction,
			Reason:      string(tc.reason),
			Details:     details,
		}

		t.Run(string(tc.reason)+" in "+tc.interaction+" replayed to the consumer", func(t *testing.T) {
			item := compatibility_checker.NewIncompatibleItem()
			item.AppendCachedBreakChange(stored)

			report := compatibility_checker.NewContractCompatibilityReport(petstoreWeb, "2.0.0", production)
			report.AppendResult(petstoreAPI, item)

			assert.Equal(t, []compatibility_checker.ContractBreakingChange{{
				Reason:  tc.reason,
				Role:    compatibility_checker.RoleConsumer,
				Details: details,
			}}, report.Hierarchical[petstoreAPI].Endpoints["/pets"]["post"][tc.interaction])
		})

		t.Run(string(tc.reason)+" in "+tc.interaction+" replayed to the provider", func(t *testing.T) {
			item := compatibility_checker.NewIncompatibleItem()
			item.AppendCachedBreakChange(stored)

			report := compatibility_checker.NewContractCompatibilityReport(petstoreAPI, "4.0.0", production)
			report.AppendResult(petstoreWeb, item)

			assert.Equal(t, []compatibility_checker.ContractBreakingChange{{
				Reason:  tc.reason,
				Role:    compatibility_checker.RoleProvider,
				Details: details,
			}}, report.Hierarchical[petstoreWeb].Endpoints["/pets"]["post"][tc.interaction])
		})
	}
}
