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
	newsletterConsumerName = "site"
	newsletterProviderName = "api"
	newsletterHash         = "post-newsletter-unsubscribe"
)

type newsletterWithoutVerdict struct{}

func (this newsletterWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

func newsletterConsumer(interaction string) model.PersistedResource {
	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	return model.PersistedResource{
		ParticipantName:    newsletterConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(newsletterProviderName),
		ConsumerHash:       null.StringFrom("site-" + newsletterHash),
		ProviderHash:       newsletterHash,
		Endpoint:           "/newsletter/unsubscribe",
		Method:             "post",
		ResponseStatusCode: status,
	}
}

func newsletterReportCheckedFrom(
	consumer model.PersistedResource,
	providerStatuses map[string][]string,
) *compatibility_checker.ContractCompatibilityReport {
	contract := &model.PersistedContract{
		ParticipantName: newsletterConsumerName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{consumer.PrimaryHash(): consumer},
	}

	return compatibility_checker.NewCompatibilityChecker(newsletterWithoutVerdict{}).Check(
		context.Background(),
		contract,
		model.NewEnvironment("production"),
		model.ResourceCounterparts{ProviderStatuses: providerStatuses},
	)
}

func TestConsumerResourceMissingFromTheDeployedProvider(t *testing.T) {
	cases := []struct {
		name             string
		interaction      string
		providerStatuses map[string][]string
		leaf             string
	}{
		{
			name:             "status the provider does not answer on that method",
			interaction:      "200",
			providerStatuses: map[string][]string{newsletterHash: {"204"}},
			leaf:             `{"reason":"provider_resource_interaction_not_found","role":"consumer","details":{"providerStatuses":"204"}}`,
		},
		{
			name:             "request the provider does not declare on that method",
			interaction:      "request",
			providerStatuses: map[string][]string{newsletterHash: {"200", "204"}},
			leaf:             `{"reason":"provider_resource_interaction_not_found","role":"consumer","details":{"providerStatuses":"200, 204"}}`,
		},
		{
			name:             "status on a method the provider declares with a request only",
			interaction:      "200",
			providerStatuses: map[string][]string{newsletterHash: nil},
			leaf:             `{"reason":"provider_resource_interaction_not_found","role":"consumer"}`,
		},
		{
			name:             "endpoint or method the provider does not declare",
			interaction:      "200",
			providerStatuses: map[string][]string{},
			leaf:             `{"reason":"provider_resource_not_found","role":"consumer"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := newsletterReportCheckedFrom(newsletterConsumer(tc.interaction), tc.providerStatuses)

			result, err := json.Marshal(report.Hierarchical[newsletterProviderName])
			require.NoError(t, err)

			assert.False(t, report.Deployable())
			assert.JSONEq(t,
				`{"deployable":false,"participantVersion":null,"endpoints":{"/newsletter/unsubscribe":{"post":{"`+tc.interaction+`":[`+tc.leaf+`]}}}}`,
				string(result),
			)
		})
	}
}

func TestProviderResourceInteractionNotFound_IsNeverAStoredVerdict(t *testing.T) {
	consumer := newsletterConsumer("200")

	interactionNotFound := compatibility_checker.NewProviderResourceInteractionNotFound(&consumer, []string{"204"})

	assert.False(t, interactionNotFound.IsPropertyBreak())
}
