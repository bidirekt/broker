package model_test

import (
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestBreakingReason_IsPropertyBreak_OnlyForPropertyReasons(t *testing.T) {
	reasons := []model.BreakingReason{
		model.ReasonProviderResourceNotFound,
		model.ReasonProviderResourceInteractionNotFound,
		model.ReasonPropertyMissingInProvider,
		model.ReasonPropertyMissingInConsumer,
		model.ReasonPropertyTypeMismatch,
		model.ReasonPropertyOptionalInProviderRequiredInConsumer,
		model.ReasonPropertyOptionalInConsumerRequiredInProvider,
		model.ReasonPropertyNullableInProviderNotNullableInConsumer,
		model.ReasonPropertyNullableInConsumerNotNullableInProvider,
		model.ReasonProviderResourceNotDeployedInEnvironment,
		model.ReasonProviderResourceRemovedButStillConsumed,
		model.ReasonProviderResourceInteractionRemovedButStillConsumed,
	}

	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			assert.Equal(t, strings.HasPrefix(string(reason), "property_"), reason.IsPropertyBreak())
		})
	}
}

func TestBreakingReason_IsPropertyBreak_FalseForAResourceReasonOfAnyProtocol(t *testing.T) {
	assert.False(t, model.BreakingReason("provider_resource_rpc_not_found").IsPropertyBreak())
}
