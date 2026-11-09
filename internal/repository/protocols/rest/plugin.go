package rest

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	insertQuery = `
		INSERT INTO rest_resources
			(resource_id, protocol, endpoint, method, response_status_code)
		VALUES
			($1, 'rest', $2, $3, NULLIF($4, ''))
	`

	loadIdentitiesQuery = `
		SELECT
			resource_id,
			endpoint,
			method,
			COALESCE(response_status_code, '')
		FROM
			rest_resources
		WHERE
			resource_id = ANY($1)
	`

	deployedProviderStatusesQuery = `
		SELECT
			COUNT(*) > 0,
			array_agg(rr.response_status_code ORDER BY rr.response_status_code COLLATE "C")
				FILTER (WHERE rr.response_status_code IS NOT NULL)
		FROM
			participants pa
		JOIN
			current_deployments cd ON cd.participant_id = pa.id AND cd.environment_id = $4
		JOIN
			resources r ON r.participant_id = pa.id
		JOIN
			rest_resources rr ON rr.resource_id = r.id
		JOIN LATERAL (
			SELECT change_type
			FROM resource_versions
			WHERE resource_id = r.id
			  AND contract_id <= cd.contract_id
			ORDER BY contract_id DESC
			LIMIT 1
		) rv ON true
		WHERE
			r.direction = 'provides'
		AND
			pa.name = $1
		AND
			rr.endpoint = $2
		AND
			rr.method = $3
		AND
			rv.change_type = 'added'
	`
)

type Plugin struct{}

func New() Plugin {
	return Plugin{}
}

func (this Plugin) Protocol() model.Protocol {
	return restmodel.Protocol
}

func (this Plugin) Insert(ctx context.Context, q repository.Querier, resourceID int64, identity model.ResourceIdentity) {
	restIdentity := identity.(restmodel.Identity)

	if _, err := q.Exec(ctx, insertQuery, resourceID, restIdentity.Endpoint, restIdentity.Method, restIdentity.Status); err != nil {
		panic(fmt.Errorf("error inserting rest resource: %w", err))
	}
}

func (this Plugin) LoadIdentities(ctx context.Context, q repository.Querier, resourceIDs []int64) map[int64]model.ResourceIdentity {
	rows, err := q.Query(ctx, loadIdentitiesQuery, resourceIDs)
	if err != nil {
		panic(fmt.Errorf("error loading rest resources: %w", err))
	}
	defer rows.Close()

	identities := make(map[int64]model.ResourceIdentity, len(resourceIDs))

	for rows.Next() {
		var resourceID int64
		var identity restmodel.Identity

		if err := rows.Scan(&resourceID, &identity.Endpoint, &identity.Method, &identity.Status); err != nil {
			panic(fmt.Errorf("error scanning rest resource: %w", err))
		}

		identities[resourceID] = identity
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading rest resources: %w", err))
	}

	return identities
}

// LoadProviderHint finds the statuses the deployed provider declares on the endpoint and
// method of a consumer resource it has no exact match for.
func (this Plugin) LoadProviderHint(
	ctx context.Context,
	q repository.Querier,
	consumer model.PersistedResource,
	environmentID int64,
) (any, bool) {
	restIdentity := consumer.Identity.(restmodel.Identity)

	var declaresMethod bool
	var statuses restmodel.DeployedStatuses

	if err := q.QueryRow(
		ctx,
		deployedProviderStatusesQuery,
		consumer.ConsumedProvider.String,
		restIdentity.Endpoint,
		restIdentity.Method,
		environmentID,
	).Scan(&declaresMethod, &statuses); err != nil {
		panic(fmt.Errorf("error loading statuses of deployed provider: %w", err))
	}

	return statuses, declaresMethod
}
