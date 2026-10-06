package compatibility_checker

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
)

func (c *CompatibilityChecker) checkProvider(
	ctx context.Context,
	providerResource model.PersistedResource,
	counterparts model.ResourceCounterparts,
	pairs *checkedPairs,
	report *ContractCompatibilityReport,
) {
	consumers := counterparts.Consumers[providerResource.ProviderHash]

	for _, consumerResource := range consumers {
		result := NewIncompatibleItem()
		result.IncompatibleCounterpart.ParticipantID = consumerResource.ParticipantID
		result.IncompatibleCounterpart.ContractID = consumerResource.ContractID
		result.IncompatibleCounterpart.ParticipantVersion = consumerResource.ParticipantVersion
		result.IncompatibleCounterpart.ParticipantName = consumerResource.ParticipantName

		pair := pairs.resolve(ctx, consumerResource.ContractID)

		if pair.hit() {
			result.VerdictCached = true
			pair.replay(result)
		} else {
			for _, breakingChange := range checkResources(&providerResource, &consumerResource) {
				result.AppendContractBreakChange(breakingChange)
			}
		}

		report.AppendResult(consumerResource.ParticipantName, result)
	}
}
