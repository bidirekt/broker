package compatibility_checker

import "github.com/bidirekt/broker/internal/model"

// checkRemovedProvider runs outside the pair cache: the pair verdict may have been stored by
// the consumer's own check, which never sees the removal, so it is always resolved live.
func (this *CompatibilityChecker) checkRemovedProvider(
	removedResource model.PersistedResource,
	checkedResources map[string]model.PersistedResource,
	counterparts model.ResourceCounterparts,
	report *ContractCompatibilityReport,
) {
	consumers := counterparts.Consumers[removedResource.ProviderHash]

	for _, result := range this.CheckRemovedProviderResource(&removedResource, checkedResources, consumers) {
		report.AppendResult(result.Counterpart.ParticipantName, result)
	}
}

func (this *CompatibilityChecker) CheckRemovedProviderResource(
	removedResource *model.PersistedResource,
	checkedResources map[string]model.PersistedResource,
	consumers []model.PersistedResource,
) []*CounterpartResult {
	results := make([]*CounterpartResult, 0, len(consumers))
	reason, details, refined := this.hooksOf(removedResource.Protocol).RefineRemoved(removedResource, checkedResources)

	for _, consumerResource := range consumers {
		// ContractID is left zeroed so this break alone never persists a verdict for the pair.
		result := NewCounterpartResult()
		result.Counterpart.ParticipantID = consumerResource.ParticipantID
		result.Counterpart.ParticipantName = consumerResource.ParticipantName
		result.Counterpart.ParticipantVersion = consumerResource.ParticipantVersion
		result.Counterpart.Role = model.CounterpartConsumer

		removal := NewProviderResourceRemovedButStillConsumed(removedResource, &consumerResource)
		removal.refine(reason, details, refined)

		result.AppendContractBreakChange(removal)

		results = append(results, result)
	}

	return results
}
