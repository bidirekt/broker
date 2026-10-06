package compatibility_checker

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

func (c *CompatibilityChecker) checkConsumer(
	ctx context.Context,
	consumerResource model.PersistedResource,
	environment *model.Environment,
	counterparts model.ResourceCounterparts,
	pairs *checkedPairs,
	report *ContractCompatibilityReport,
) {
	providerResource, found := counterparts.Providers[consumerResource.ProviderHash]

	incompatibleItem := NewIncompatibleItem()

	if !found {
		incompatibleItem.AppendContractBreakChange(
			NewProviderResourceNotFound(&consumerResource),
		)

		report.AppendResult(consumerResource.ConsumedProvider.String, incompatibleItem)

		return
	}

	version, deployed := providerResource.DeployedVersionIn(environment.Name)
	if !deployed {
		incompatibleItem.IncompatibleCounterpart.ParticipantID = providerResource.ParticipantID

		incompatibleItem.AppendContractBreakChange(NewProviderResourceNotDeployed(
			&consumerResource,
			providerResource.DeployedEnvironments(),
		))

		report.AppendResult(consumerResource.ConsumedProvider.String, incompatibleItem)

		return
	}

	providerResource.ParticipantVersion = null.StringFrom(version)
	incompatibleItem.IncompatibleCounterpart.ParticipantID = providerResource.ParticipantID
	incompatibleItem.IncompatibleCounterpart.ContractID = providerResource.ContractID
	incompatibleItem.IncompatibleCounterpart.ParticipantVersion = providerResource.ParticipantVersion
	incompatibleItem.IncompatibleCounterpart.ParticipantName = providerResource.ParticipantName

	pair := pairs.resolve(ctx, providerResource.ContractID)

	if pair.hit() {
		incompatibleItem.VerdictCached = true
		pair.replay(incompatibleItem)
	} else {
		for _, breakingChange := range checkResources(&consumerResource, &providerResource) {
			incompatibleItem.AppendContractBreakChange(breakingChange)
		}
	}

	report.AppendResult(consumerResource.ConsumedProvider.String, incompatibleItem)
}
