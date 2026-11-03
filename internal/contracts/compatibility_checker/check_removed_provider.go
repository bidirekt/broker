package compatibility_checker

import (
	"slices"

	"github.com/bidirekt/broker/internal/model"
)

// checkRemovedProvider runs outside the pair cache: the pair verdict may have been stored by
// the consumer's own check, which never sees the removal, so it is always resolved live.
func (this *CompatibilityChecker) checkRemovedProvider(
	removedResource model.PersistedResource,
	checkedResources map[string]model.PersistedResource,
	counterparts model.ResourceCounterparts,
	report *ContractCompatibilityReport,
) {
	consumers := counterparts.Consumers[removedResource.ProviderHash]

	for _, result := range CheckRemovedProviderResource(&removedResource, checkedResources, consumers) {
		report.AppendResult(result.IncompatibleCounterpart.ParticipantName, result)
	}
}

func CheckRemovedProviderResource(
	removedResource *model.PersistedResource,
	checkedResources map[string]model.PersistedResource,
	consumers []model.PersistedResource,
) []*IncompatibleItem {
	results := make([]*IncompatibleItem, 0, len(consumers))
	providerStatuses, methodRemains := remainingProviderStatuses(removedResource, checkedResources)

	for _, consumerResource := range consumers {
		// ContractID is left zeroed so this break alone never persists a verdict for the pair.
		result := NewIncompatibleItem()
		result.IncompatibleCounterpart.ParticipantID = consumerResource.ParticipantID
		result.IncompatibleCounterpart.ParticipantName = consumerResource.ParticipantName
		result.IncompatibleCounterpart.ParticipantVersion = consumerResource.ParticipantVersion

		removal := NewProviderResourceRemovedButStillConsumed(removedResource, &consumerResource)
		if methodRemains {
			removal = NewProviderResourceInteractionRemovedButStillConsumed(
				removedResource,
				&consumerResource,
				providerStatuses,
			)
		}

		result.AppendContractBreakChange(removal)

		results = append(results, result)
	}

	return results
}

// A removed request keeps no statuses: its endpoint and method only remain through responses.
func remainingProviderStatuses(
	removedResource *model.PersistedResource,
	checkedResources map[string]model.PersistedResource,
) ([]string, bool) {
	var statuses []string
	methodRemains := false

	for _, resource := range checkedResources {
		if resource.Removed ||
			!resource.IsProvider() ||
			resource.Endpoint != removedResource.Endpoint ||
			resource.Method != removedResource.Method {
			continue
		}

		methodRemains = true

		if removedResource.Interaction == model.RestResponse && resource.Interaction == model.RestResponse {
			statuses = append(statuses, resource.ResponseStatusCode.String)
		}
	}

	slices.Sort(statuses)

	return statuses, methodRemains
}
