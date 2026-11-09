package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
)

const (
	deployedResourcesByProviderHashAndEnvironmentQuery = `
		SELECT
			cd.contract_id,
			cd.version,` + resourceColumns + `
		FROM
			resources r
		JOIN
			participants pa ON pa.id = r.participant_id
		JOIN
			current_deployments cd ON cd.participant_id = r.participant_id AND cd.environment_id = $3
		JOIN LATERAL (
			SELECT change_type
			FROM resource_versions
			WHERE resource_id = r.id
			  AND contract_id <= cd.contract_id
			ORDER BY contract_id DESC
			LIMIT 1
		) rv ON true
		JOIN LATERAL (
			SELECT
				array_agg(rb.source ORDER BY rb.source COLLATE "C", rb.id) AS sources,
				array_agg(s.name ORDER BY rb.source COLLATE "C", rb.id) AS schema_names
			FROM resource_bodies rb
			LEFT JOIN schemas s ON s.id = rb.schema_id
			WHERE rb.resource_id = r.id AND rb.contract_id = cd.contract_id
		) bodies ON true
		WHERE
			r.direction = $1
		AND
			r.provider_hash = $2
		AND
			rv.change_type = 'added'
	`

	registeredProviderQuery = `
		SELECT
			pa.id,
			cd.version
		FROM
			participants pa
		LEFT JOIN
			current_deployments cd ON cd.participant_id = pa.id AND cd.environment_id = $2
		WHERE
			pa.name = $1
	`
)

// LoadCounterparts loads the deployed counterparts of the contract's resources in the environment, once per hash and side.
func (this *ContractRepository) LoadCounterparts(
	ctx context.Context,
	contract *model.PersistedContract,
	environmentID int64,
) model.ResourceCounterparts {
	counterparts := model.ResourceCounterparts{
		Providers:           make(map[string]model.PersistedResource),
		Consumers:           make(map[string][]model.PersistedResource),
		ProviderHints:       make(map[string]any),
		RegisteredProviders: make(map[string]model.RegisteredProvider),
	}

	providersQueried := make(map[string]bool)
	providerNamesQueried := make(map[string]bool)
	catalogs := schemaCatalogs{}

	for _, resource := range contract.Resources {
		switch resource.Direction {
		case model.Consumes:
			if resource.Removed || providersQueried[resource.ProviderHash] {
				continue
			}

			providersQueried[resource.ProviderHash] = true
			providerName := resource.ConsumedProvider.String

			if !providerNamesQueried[providerName] {
				providerNamesQueried[providerName] = true

				if registered, isParticipant := this.loadRegisteredProvider(ctx, providerName, environmentID); isParticipant {
					counterparts.RegisteredProviders[providerName] = registered
				}
			}

			if !counterparts.RegisteredProviders[providerName].DeployedVersion.Valid {
				continue
			}

			if provider, found := this.loadProviderResource(ctx, catalogs, resource.ProviderHash, environmentID); found {
				counterparts.Providers[resource.ProviderHash] = provider

				continue
			}

			if hint, found := this.tables[resource.Protocol].LoadProviderHint(ctx, this.pool, resource, environmentID); found {
				counterparts.ProviderHints[resource.ProviderHash] = hint
			}
		case model.Provides:
			if _, queried := counterparts.Consumers[resource.ProviderHash]; queried {
				continue
			}

			counterparts.Consumers[resource.ProviderHash] = this.loadDeployedResources(ctx, catalogs, model.Consumes, resource.ProviderHash, environmentID)
		}
	}

	return counterparts
}

func (this *ContractRepository) GetProviderResourceByProviderHashAndEnvironmentID(
	ctx context.Context,
	providerHash string,
	environmentID int64,
) (model.PersistedResource, bool) {
	return this.loadProviderResource(ctx, schemaCatalogs{}, providerHash, environmentID)
}

func (this *ContractRepository) GetConsumersResourcesByProviderHashAndEnvironmentID(
	ctx context.Context,
	providerHash string,
	environmentID int64,
) []model.PersistedResource {
	return this.loadDeployedResources(ctx, schemaCatalogs{}, model.Consumes, providerHash, environmentID)
}

func (this *ContractRepository) loadProviderResource(
	ctx context.Context,
	catalogs schemaCatalogs,
	providerHash string,
	environmentID int64,
) (model.PersistedResource, bool) {
	providers := this.loadDeployedResources(ctx, catalogs, model.Provides, providerHash, environmentID)
	if len(providers) == 0 {
		return model.PersistedResource{}, false
	}

	return providers[0], true
}

func (this *ContractRepository) loadRegisteredProvider(
	ctx context.Context,
	participantName string,
	environmentID int64,
) (model.RegisteredProvider, bool) {
	var registered model.RegisteredProvider

	err := this.pool.QueryRow(ctx, registeredProviderQuery, participantName, environmentID).
		Scan(&registered.ParticipantID, &registered.DeployedVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RegisteredProvider{}, false
	}
	if err != nil {
		panic(fmt.Errorf("error loading registered provider: %w", err))
	}

	return registered, true
}

func (this *ContractRepository) loadDeployedResources(
	ctx context.Context,
	catalogs schemaCatalogs,
	direction model.Direction,
	providerHash string,
	environmentID int64,
) []model.PersistedResource {
	rows, err := this.pool.Query(
		ctx,
		deployedResourcesByProviderHashAndEnvironmentQuery,
		string(direction),
		providerHash,
		environmentID,
	)
	if err != nil {
		panic(fmt.Errorf("error loading deployed resources by provider hash: %w", err))
	}

	tableRows := scanTableRows(rows)
	identities := this.identitiesOf(ctx, this.pool, tableRows)
	resources := make([]model.PersistedResource, len(tableRows))

	for index, row := range tableRows {
		resources[index] = row.toResourceModel(identities[row.ResourceID])
		resources[index].ParticipantVersion = null.StringFrom(row.ContractVersion)
	}

	for index := range resources {
		resources[index].Schemas = this.loadSchemasAt(ctx, this.pool, catalogs, resources[index].ContractID)
	}

	return resources
}
