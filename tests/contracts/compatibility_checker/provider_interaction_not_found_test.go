package compatibility_checker_test

import (
	"context"
	"encoding/json"
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
	newsletterConsumerName = "site"
	newsletterProviderName = "api"
	newsletterHash         = "post-newsletter-unsubscribe"
)

type newsletterWithoutVerdict struct{}

func (this newsletterWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

func newsletterConsumer(interaction string) model.PersistedResource {
	body, status := model.Output, interaction
	if interaction == "request" {
		body, status = model.Input, ""
	}

	return model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  newsletterConsumerName,
			Direction:        model.Consumes,
			Protocol:         restmodel.Protocol,
			Body:             body,
			Identity:         restmodel.Identity{Endpoint: "/newsletter/unsubscribe", Method: "post", Status: status},
			ConsumedProvider: null.StringFrom(newsletterProviderName),
		},
		ConsumerHash: null.StringFrom("site-" + newsletterHash),
		ProviderHash: newsletterHash,
	}
}

func newsletterReportCheckedFrom(
	consumer model.PersistedResource,
	providerHints map[string]any,
) *compatibility_checker.ContractCompatibilityReport {
	contract := &model.PersistedContract{
		ParticipantName: newsletterConsumerName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{consumer.PrimaryHash(): consumer},
	}

	return compatibility_checker.NewCompatibilityChecker(newsletterWithoutVerdict{}, []compatibility_checker.ProtocolHooks{restcontracts.New()}).Check(
		context.Background(),
		contract,
		model.NewEnvironment("production"),
		model.ResourceCounterparts{
			ProviderHints:       providerHints,
			RegisteredProviders: map[string]model.RegisteredProvider{newsletterProviderName: {ParticipantID: 2, DeployedVersion: null.StringFrom("1.0.0")}},
		},
	)
}

func TestConsumerResourceMissingFromTheDeployedProvider(t *testing.T) {
	cases := []struct {
		name          string
		interaction   string
		providerHints map[string]any
		leaf          string
	}{
		{
			name:          "status the provider does not answer on that method",
			interaction:   "200",
			providerHints: map[string]any{newsletterHash: restmodel.DeployedStatuses{"204"}},
			leaf:          `{"reason":"provider_resource_interaction_not_found","role":"consumer","details":{"providerStatuses":"204"}}`,
		},
		{
			name:          "request the provider does not declare on that method",
			interaction:   "request",
			providerHints: map[string]any{newsletterHash: restmodel.DeployedStatuses{"200", "204"}},
			leaf:          `{"reason":"provider_resource_interaction_not_found","role":"consumer","details":{"providerStatuses":"200, 204"}}`,
		},
		{
			name:          "status on a method the provider declares with a request only",
			interaction:   "200",
			providerHints: map[string]any{newsletterHash: restmodel.DeployedStatuses(nil)},
			leaf:          `{"reason":"provider_resource_interaction_not_found","role":"consumer"}`,
		},
		{
			name:          "endpoint or method the provider does not declare",
			interaction:   "200",
			providerHints: map[string]any{},
			leaf:          `{"reason":"provider_resource_not_found","role":"consumer"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := newsletterReportCheckedFrom(newsletterConsumer(tc.interaction), tc.providerHints)

			result, err := json.Marshal(report.Hierarchical[newsletterProviderName])
			require.NoError(t, err)

			assert.False(t, report.Deployable())
			assert.JSONEq(t,
				`{"deployable":false,"participantVersion":"1.0.0","endpoints":{"/newsletter/unsubscribe":{"post":{"`+tc.interaction+`":[`+tc.leaf+`]}}}}`,
				string(result),
			)
		})
	}
}

func TestProviderResourceInteractionNotFound_IsNeverAStoredVerdict(t *testing.T) {
	report := newsletterReportCheckedFrom(
		newsletterConsumer("200"),
		map[string]any{newsletterHash: restmodel.DeployedStatuses{"204"}},
	)

	result := report.Results[newsletterProviderName]
	require.Len(t, result.Breaks, 1)
	interactionNotFound := result.Breaks[0]

	assert.Equal(t, model.ReasonProviderResourceInteractionNotFound, interactionNotFound.Reason)
	assert.False(t, interactionNotFound.IsPropertyBreak())
	assert.Empty(t, compatibility_checker.ToVerdictBreaks(result))
}
