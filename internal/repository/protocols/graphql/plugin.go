package graphql

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	insertQuery = `
		INSERT INTO graphql_resources
			(resource_id, protocol, operation, field, interaction)
		VALUES
			($1, 'graphql', $2, $3, $4)
	`

	loadIdentitiesQuery = `
		SELECT
			resource_id,
			operation,
			field,
			interaction
		FROM
			graphql_resources
		WHERE
			resource_id = ANY($1)
	`
)

type Plugin struct{}

func New() Plugin {
	return Plugin{}
}

func (this Plugin) Protocol() model.Protocol {
	return graphqlmodel.Protocol
}

func (this Plugin) Insert(ctx context.Context, q repository.Querier, resourceID int64, identity model.ResourceIdentity) {
	graphqlIdentity := identity.(graphqlmodel.Identity)

	if _, err := q.Exec(ctx, insertQuery, resourceID, graphqlIdentity.Operation, graphqlIdentity.Field, graphqlIdentity.Interaction); err != nil {
		panic(fmt.Errorf("error inserting graphql resource: %w", err))
	}
}

func (this Plugin) LoadIdentities(ctx context.Context, q repository.Querier, resourceIDs []int64) map[int64]model.ResourceIdentity {
	rows, err := q.Query(ctx, loadIdentitiesQuery, resourceIDs)
	if err != nil {
		panic(fmt.Errorf("error loading graphql resources: %w", err))
	}
	defer rows.Close()

	identities := make(map[int64]model.ResourceIdentity, len(resourceIDs))

	for rows.Next() {
		var resourceID int64
		var identity graphqlmodel.Identity

		if err := rows.Scan(&resourceID, &identity.Operation, &identity.Field, &identity.Interaction); err != nil {
			panic(fmt.Errorf("error scanning graphql resource: %w", err))
		}

		identities[resourceID] = identity
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading graphql resources: %w", err))
	}

	return identities
}
