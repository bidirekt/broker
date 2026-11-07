package compatibility_checker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rolesEnvironment = "production"
	rolesHash        = "get-orders-204"
)

type rolesWithoutVerdict struct{}

func (this rolesWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

func rolesProvider(participantID int64, participantName string) model.PersistedResource {
	return model.PersistedResource{
		ParticipantID:      participantID,
		ParticipantName:    participantName,
		ContractID:         participantID * 10,
		Direction:          model.Provides,
		Interaction:        model.RestResponse,
		ProviderHash:       rolesHash,
		Endpoint:           "/orders",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("204"),
		ParticipantVersion: null.StringFrom("v1"),
	}
}

func rolesConsumer(participantID int64, participantName, providerName string) model.PersistedResource {
	return model.PersistedResource{
		ParticipantID:      participantID,
		ParticipantName:    participantName,
		ContractID:         participantID * 10,
		Direction:          model.Consumes,
		Interaction:        model.RestResponse,
		ConsumedProvider:   null.StringFrom(providerName),
		ConsumerHash:       null.StringFrom(participantName + "-" + rolesHash),
		ProviderHash:       rolesHash,
		Endpoint:           "/orders",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("204"),
		ParticipantVersion: null.StringFrom("v1"),
	}
}

func rolesReportCheckedFrom(
	participantName string,
	resources []model.PersistedResource,
	counterparts model.ResourceCounterparts,
) *compatibility_checker.ContractCompatibilityReport {
	contract := &model.PersistedContract{
		ID:              1,
		ParticipantName: participantName,
		Version:         "v2",
		Resources:       make(map[string]model.PersistedResource, len(resources)),
	}

	for _, resource := range resources {
		contract.Resources[resource.PrimaryHash()] = resource
	}

	return compatibility_checker.NewCompatibilityChecker(rolesWithoutVerdict{}).Check(
		context.Background(),
		contract,
		model.NewEnvironment(rolesEnvironment),
		counterparts,
	)
}

func TestCompatibleProviderOfTheCheckedConsumerHasTheProviderRole(t *testing.T) {
	report := rolesReportCheckedFrom("front", []model.PersistedResource{rolesConsumer(1, "front", "orders")},
		model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{rolesHash: rolesProvider(2, "orders")},
			RegisteredProviders: map[string]model.RegisteredProvider{"orders": {ParticipantID: 2, DeployedVersion: null.StringFrom("v1")}},
		})

	assert.True(t, report.Results["orders"].Deployable)
	assert.Equal(t, model.CounterpartProvider, report.Results["orders"].Counterpart.Role)
}

func TestMissingProviderResourceHasTheProviderRole(t *testing.T) {
	report := rolesReportCheckedFrom("front", []model.PersistedResource{rolesConsumer(1, "front", "orders")},
		model.ResourceCounterparts{})

	assert.False(t, report.Results["orders"].Deployable)
	assert.Equal(t, model.CounterpartProvider, report.Results["orders"].Counterpart.Role)
}

func TestCompatibleConsumerOfTheCheckedProviderHasTheConsumerRole(t *testing.T) {
	report := rolesReportCheckedFrom("orders", []model.PersistedResource{rolesProvider(2, "orders")},
		model.ResourceCounterparts{Consumers: map[string][]model.PersistedResource{rolesHash: {rolesConsumer(1, "front", "orders")}}})

	assert.True(t, report.Results["front"].Deployable)
	assert.Equal(t, model.CounterpartConsumer, report.Results["front"].Counterpart.Role)
}

func TestConsumerOfARemovedProviderResourceHasTheConsumerRole(t *testing.T) {
	removed := rolesProvider(2, "orders")
	removed.Removed = true

	report := rolesReportCheckedFrom("orders", []model.PersistedResource{removed},
		model.ResourceCounterparts{Consumers: map[string][]model.PersistedResource{rolesHash: {rolesConsumer(1, "front", "orders")}}})

	assert.False(t, report.Results["front"].Deployable)
	assert.Equal(t, model.CounterpartConsumer, report.Results["front"].Counterpart.Role)
}

func TestSelfConsumptionHasBothRoles(t *testing.T) {
	provider := rolesProvider(2, "orders")
	consumer := rolesConsumer(2, "orders", "orders")

	report := rolesReportCheckedFrom("orders", []model.PersistedResource{provider, consumer},
		model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{rolesHash: provider},
			Consumers:           map[string][]model.PersistedResource{rolesHash: {consumer}},
			RegisteredProviders: map[string]model.RegisteredProvider{"orders": {ParticipantID: 2, DeployedVersion: null.StringFrom("v1")}},
		})

	assert.Equal(t, model.CounterpartBoth, report.Results["orders"].Counterpart.Role)
}

func TestAppendResultMergesTheRolesOfOneCounterpart(t *testing.T) {
	cases := []struct {
		name     string
		roles    []model.CounterpartRole
		expected model.CounterpartRole
	}{
		{"provider only", []model.CounterpartRole{model.CounterpartProvider, model.CounterpartProvider}, model.CounterpartProvider},
		{"consumer only", []model.CounterpartRole{model.CounterpartConsumer, model.CounterpartConsumer}, model.CounterpartConsumer},
		{"provider then consumer", []model.CounterpartRole{model.CounterpartProvider, model.CounterpartConsumer}, model.CounterpartBoth},
		{"consumer then provider", []model.CounterpartRole{model.CounterpartConsumer, model.CounterpartProvider}, model.CounterpartBoth},
		{"both then provider", []model.CounterpartRole{model.CounterpartProvider, model.CounterpartConsumer, model.CounterpartProvider}, model.CounterpartBoth},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := compatibility_checker.NewContractCompatibilityReport("orders", "v2", rolesEnvironment)

			for _, role := range tc.roles {
				result := compatibility_checker.NewCounterpartResult()
				result.Counterpart.Role = role
				report.AppendResult("front", result)
			}

			assert.Equal(t, tc.expected, report.Results["front"].Counterpart.Role)
		})
	}
}

func TestCounterpartRoleStaysOffTheWire(t *testing.T) {
	report := rolesReportCheckedFrom("front", []model.PersistedResource{rolesConsumer(1, "front", "orders")},
		model.ResourceCounterparts{
			Providers:           map[string]model.PersistedResource{rolesHash: rolesProvider(2, "orders")},
			RegisteredProviders: map[string]model.RegisteredProvider{"orders": {ParticipantID: 2, DeployedVersion: null.StringFrom("v1")}},
		})

	wire, err := json.Marshal(report.Hierarchical)
	require.NoError(t, err)

	assert.JSONEq(t, `{"orders":{"deployable":true,"participantVersion":"v1","endpoints":{}}}`, string(wire))
}

func TestUnmatchedRegisteredProviderKeepsItsIdentity(t *testing.T) {
	cases := []struct {
		name            string
		deployedVersion null.String
		reason          model.BreakingReason
	}{
		{"deployed in the environment", null.StringFrom("v7"), model.ReasonProviderResourceNotFound},
		{"not deployed in the environment", null.String{}, model.ReasonProviderResourceNotDeployedInEnvironment},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := rolesReportCheckedFrom("front", []model.PersistedResource{rolesConsumer(1, "front", "orders")},
				model.ResourceCounterparts{RegisteredProviders: map[string]model.RegisteredProvider{
					"orders": {ParticipantID: 2, DeployedVersion: tc.deployedVersion},
				}})

			result := report.Results["orders"]
			assert.Equal(t, int64(2), result.Counterpart.ParticipantID)
			assert.Equal(t, tc.deployedVersion, result.Counterpart.ParticipantVersion)
			assert.Zero(t, result.Counterpart.ContractID)
			assert.Equal(t, tc.deployedVersion, report.Hierarchical["orders"].Version)
			require.Len(t, result.Breaks, 1)
			assert.Equal(t, tc.reason, result.Breaks[0].Reason)
			assert.Nil(t, result.Breaks[0].Details)
		})
	}
}

func TestUnmatchedNameThatIsNotAParticipantIsNotDeployedWithNoIdentity(t *testing.T) {
	report := rolesReportCheckedFrom("front", []model.PersistedResource{rolesConsumer(1, "front", "orders")},
		model.ResourceCounterparts{})

	result := report.Results["orders"]
	assert.Zero(t, result.Counterpart.ParticipantID)
	assert.False(t, result.Counterpart.ParticipantVersion.Valid)
	require.Len(t, result.Breaks, 1)
	assert.Equal(t, model.ReasonProviderResourceNotDeployedInEnvironment, result.Breaks[0].Reason)
	assert.Nil(t, result.Breaks[0].Details)
}
