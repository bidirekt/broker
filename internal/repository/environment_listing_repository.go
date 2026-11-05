package repository

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	countEnvironmentListingsQuery = `
		SELECT
			COUNT(*)
		FROM
			environments
	`

	listEnvironmentListingsQuery = `
		SELECT
			e.id,
			e.name,
			(SELECT COUNT(*) FROM current_deployments c WHERE c.environment_id = e.id),
			p.name,
			d.version,
			d.deployed_at
		FROM
			environments e
			LEFT JOIN LATERAL (
				SELECT
					participant_id, version, deployed_at
				FROM
					deployments
				WHERE
					environment_id = e.id
				ORDER BY
					deployed_at DESC, id DESC
				LIMIT 1
			) d ON true
			LEFT JOIN participants p ON p.id = d.participant_id
		ORDER BY
			e.name COLLATE "C"
		LIMIT $1 OFFSET $2
	`
)

type EnvironmentListingRepository struct {
	pool *pgxpool.Pool
}

func NewEnvironmentListingRepository(pool *pgxpool.Pool) *EnvironmentListingRepository {
	return &EnvironmentListingRepository{pool: pool}
}

func (this *EnvironmentListingRepository) List(ctx context.Context, limit, offset int) (environments []*model.EnvironmentListing, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countEnvironmentListingsQuery).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting environments: %w", err))
	}

	rows, err := tx.Query(ctx, listEnvironmentListingsQuery, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing environments: %w", err))
	}

	environments, err = pgx.AppendRows(make([]*model.EnvironmentListing, 0), rows, func(row pgx.CollectableRow) (*model.EnvironmentListing, error) {
		environment := &model.EnvironmentListing{}
		err := row.Scan(&environment.ID, &environment.Name, &environment.DeployedParticipantCount,
			&environment.LastDeploymentParticipant, &environment.LastDeploymentVersion, &environment.LastDeployedAt)
		return environment, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing environments: %w", err))
	}

	return environments, total
}
