package compatibility_checker

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

func (this *CompatibilityChecker) checkConsumer(
	ctx context.Context,
	consumerResource model.PersistedResource,
	environment *model.Environment,
	counterparts model.ResourceCounterparts,
	pairs *checkedPairs,
	report *ContractCompatibilityReport,
) {
	providerResource, found := counterparts.Providers[consumerResource.ProviderHash]

	result := NewCounterpartResult()

	if !found {
		notFound := NewProviderResourceNotFound(&consumerResource)
		if providerStatuses, declaresMethod := counterparts.ProviderStatuses[consumerResource.ProviderHash]; declaresMethod {
			notFound = NewProviderResourceInteractionNotFound(&consumerResource, providerStatuses)
		}

		result.AppendContractBreakChange(notFound)

		report.AppendResult(consumerResource.ConsumedProvider.String, result)

		return
	}

	version, deployed := providerResource.DeployedVersionIn(environment.Name)
	if !deployed {
		result.Counterpart.ParticipantID = providerResource.ParticipantID

		result.AppendContractBreakChange(NewProviderResourceNotDeployed(
			&consumerResource,
			providerResource.DeployedEnvironments(),
		))

		report.AppendResult(consumerResource.ConsumedProvider.String, result)

		return
	}

	providerResource.ParticipantVersion = null.StringFrom(version)
	result.Counterpart.ParticipantID = providerResource.ParticipantID
	result.Counterpart.ContractID = providerResource.ContractID
	result.Counterpart.ParticipantVersion = providerResource.ParticipantVersion
	result.Counterpart.ParticipantName = providerResource.ParticipantName

	pair := pairs.resolve(ctx, providerResource.ContractID)

	if pair.hit() {
		result.VerdictCached = true
		pair.replay(result)
	} else {
		for _, breakingChange := range checkResources(&consumerResource, &providerResource) {
			result.AppendContractBreakChange(breakingChange)
		}
	}

	report.AppendResult(consumerResource.ConsumedProvider.String, result)
}
