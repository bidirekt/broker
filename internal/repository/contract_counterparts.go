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
		Providers:          make(map[string]model.PersistedResource),
		Consumers:          make(map[string][]model.PersistedResource),
		ProviderStatuses:   make(map[string][]string),
		UnmatchedProviders: make(map[string]model.RegisteredProvider),
	}

	providersQueried := make(map[string]bool)
	providerNamesQueried := make(map[string]bool)
	registeredProviders := make(map[string]model.RegisteredProvider)
	catalogs := schemaCatalogs{}

	for _, resource := range contract.Resources {
		switch resource.Direction {
		case model.Consumes:
			if resource.Removed || providersQueried[resource.ProviderHash] {
				continue
			}

			providersQueried[resource.ProviderHash] = true

			if provider, found := this.loadProviderResource(ctx, catalogs, resource.ProviderHash, environmentID); found {
				counterparts.Providers[resource.ProviderHash] = provider

				continue
			}

			if !providerNamesQueried[resource.ConsumedProvider.String] {
				providerNamesQueried[resource.ConsumedProvider.String] = true

				if registered, isParticipant := this.loadRegisteredProvider(ctx, resource.ConsumedProvider.String, environmentID); isParticipant {
					registeredProviders[resource.ConsumedProvider.String] = registered
				}
			}

			if registered, isParticipant := registeredProviders[resource.ConsumedProvider.String]; isParticipant {
				counterparts.UnmatchedProviders[resource.ProviderHash] = registered
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
