package repository

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
)

// LoadCounterparts loads the deployed counterparts of the contract's resources in the environment, once per hash and side.
func (this *ContractRepository) LoadCounterparts(
	ctx context.Context,
	contract *model.PersistedContract,
	environmentID int64,
) model.ResourceCounterparts {
	counterparts := model.ResourceCounterparts{
		Providers:        make(map[string]model.PersistedResource),
		Consumers:        make(map[string][]model.PersistedResource),
		ProviderStatuses: make(map[string][]string),
	}

	providersQueried := make(map[string]bool)
	catalogs := schemaCatalogs{}

	for _, resource := range contract.Resources {
		switch resource.Direction {
		case model.Consumes:
			if resource.Removed || providersQueried[resource.ProviderHash] {
				continue
			}

			providersQueried[resource.ProviderHash] = true

			provider, err := this.loadProviderResource(ctx, catalogs, resource.ProviderHash, environmentID)
			if err == nil {
				counterparts.Providers[resource.ProviderHash] = provider

				continue
			}

			if statuses, declaresMethod := this.loadDeployedProviderStatuses(ctx, resource, environmentID); declaresMethod {
				counterparts.ProviderStatuses[resource.ProviderHash] = statuses
			}
		case model.Provides:
			if _, queried := counterparts.Consumers[resource.ProviderHash]; queried {
				continue
			}

			counterparts.Consumers[resource.ProviderHash] = this.loadConsumerResources(ctx, catalogs, resource.ProviderHash, environmentID)
		}
	}

	return counterparts
}
