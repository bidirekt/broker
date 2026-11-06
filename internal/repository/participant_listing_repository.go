package repository

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	countParticipantListingsQuery = `
		SELECT
			COUNT(*)
		FROM
			participants
	`

	listParticipantListingsQuery = `
		SELECT
			p.id, p.name, l.version, l.created_at
		FROM
			participants p
			LEFT JOIN latest_contract_versions l ON l.participant_id = p.id
		ORDER BY
			p.name COLLATE "C"
		LIMIT $1 OFFSET $2
	`

	listParticipantListingDeploymentsQuery = `
		SELECT
			c.participant_id, e.id, e.name, c.version
		FROM
			current_deployments c
			JOIN environments e ON e.id = c.environment_id
		WHERE
			c.participant_id = ANY($1)
		ORDER BY
			e.name COLLATE "C"
	`
)

type ParticipantListingRepository struct {
	pool *pgxpool.Pool
}

func NewParticipantListingRepository(pool *pgxpool.Pool) *ParticipantListingRepository {
	return &ParticipantListingRepository{pool: pool}
}

func (this *ParticipantListingRepository) List(ctx context.Context, limit, offset int) (participants []*model.ParticipantListing, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countParticipantListingsQuery).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting participants: %w", err))
	}

	rows, err := tx.Query(ctx, listParticipantListingsQuery, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing participants: %w", err))
	}

	participants, err = pgx.AppendRows(make([]*model.ParticipantListing, 0), rows, func(row pgx.CollectableRow) (*model.ParticipantListing, error) {
		participant := &model.ParticipantListing{}
		err := row.Scan(&participant.ID, &participant.Name, &participant.LatestVersion, &participant.PublishedAt)
		return participant, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing participants: %w", err))
	}

	byID := make(map[int64]*model.ParticipantListing, len(participants))
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		byID[participant.ID] = participant
		ids = append(ids, participant.ID)
	}

	rows, err = tx.Query(ctx, listParticipantListingDeploymentsQuery, ids)
	if err != nil {
		panic(fmt.Errorf("error listing participant deployments: %w", err))
	}

	deployments, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Participant: &model.Participant{}, Environment: &model.Environment{}}
		err := row.Scan(&deployment.Participant.ID, &deployment.Environment.ID, &deployment.Environment.Name, &deployment.Version)
		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing participant deployments: %w", err))
	}

	for _, deployment := range deployments {
		participant := byID[deployment.Participant.ID]
		participant.Deployments = append(participant.Deployments, deployment)
	}

	return participants, total
}
