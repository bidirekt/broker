package compatibility_checker

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
)

func (this *CompatibilityChecker) checkConsumer(
	ctx context.Context,
	consumerResource model.PersistedResource,
	counterparts model.ResourceCounterparts,
	pairs *checkedPairs,
	report *ContractCompatibilityReport,
) {
	providerName := consumerResource.ConsumedProvider.String
	registered := counterparts.RegisteredProviders[providerName]

	result := NewCounterpartResult()
	result.Counterpart.Role = model.CounterpartProvider
	result.Counterpart.ParticipantID = registered.ParticipantID

	if !registered.DeployedVersion.Valid {
		result.AppendContractBreakChange(NewProviderResourceNotDeployed(&consumerResource))

		report.AppendResult(providerName, result)

		return
	}

	providerResource, found := counterparts.Providers[consumerResource.ProviderHash]
	if !found {
		result.Counterpart.ParticipantVersion = registered.DeployedVersion

		notFound := NewProviderResourceNotFound(&consumerResource)
		if hint, declaresAround := counterparts.ProviderHints[consumerResource.ProviderHash]; declaresAround {
			notFound.refine(this.hooksOf(consumerResource.Protocol).RefineNotFound(&consumerResource, hint))
		}

		result.AppendContractBreakChange(notFound)

		report.AppendResult(providerName, result)

		return
	}

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

	report.AppendResult(providerName, result)
}
