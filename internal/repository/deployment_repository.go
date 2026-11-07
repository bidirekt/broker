package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	lockParticipantForDeploymentQuery = `
		SELECT 1 FROM participants WHERE id = $1 FOR NO KEY UPDATE
	`

	insertDeploymentQuery = `
		INSERT INTO deployments
			(participant_id, version, environment_id, rollback, deployed_at)
		SELECT
			$1,
			$2,
			$3,
			EXISTS (
				SELECT 1 FROM deployments
				WHERE participant_id = $1 AND environment_id = $3 AND version = $2
			),
			clock_timestamp()
		WHERE NOT EXISTS (
			SELECT 1 FROM current_deployments
			WHERE participant_id = $1 AND environment_id = $3 AND version = $2
		)
		RETURNING id, rollback, deployed_at
	`

	upsertCurrentDeploymentQuery = `
		INSERT INTO current_deployments
			(participant_id, environment_id, version, contract_id, deployed_at)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT contract_id FROM contract_versions WHERE participant_id = $1 AND version = $3),
			$4
		)
		ON CONFLICT (participant_id, environment_id) DO UPDATE SET
			version = EXCLUDED.version,
			contract_id = EXCLUDED.contract_id,
			deployed_at = EXCLUDED.deployed_at
	`

	currentDeploymentInEnvQuery = `
		SELECT version, contract_id
		FROM current_deployments
		WHERE participant_id = $1 AND environment_id = $2
	`

	listCurrentDeploymentsInEnvQuery = `
		SELECT p.id, p.name, cd.version, cd.deployed_at
		FROM current_deployments cd
		JOIN participants p ON p.id = cd.participant_id
		WHERE cd.environment_id = $1
		ORDER BY p.name COLLATE "C"
	`
)

type DeploymentRepository struct {
	pool *pgxpool.Pool
}

func NewDeploymentRepository(pool *pgxpool.Pool) *DeploymentRepository {
	return &DeploymentRepository{pool: pool}
}

func (this *DeploymentRepository) Insert(ctx context.Context, d *model.Deployment) (recorded bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, lockParticipantForDeploymentQuery, d.Participant.ID); err != nil {
		panic(fmt.Errorf("error locking participant for deployment: %w", err))
	}

	err = tx.QueryRow(
		ctx,
		insertDeploymentQuery,
		d.Participant.ID,
		d.Version,
		d.Environment.ID,
	).Scan(&d.ID, &d.Rollback, &d.DeployedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error inserting deployment: %w", err))
	}

	if _, err := tx.Exec(
		ctx,
		upsertCurrentDeploymentQuery,
		d.Participant.ID,
		d.Environment.ID,
		d.Version,
		d.DeployedAt,
	); err != nil {
		panic(fmt.Errorf("error upserting current deployment: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true
}

func (this *DeploymentRepository) CurrentDeploymentInEnv(
	ctx context.Context,
	participantID int64,
	environmentID int64,
) (version string, contractID int64, deployed bool) {
	err := this.pool.QueryRow(ctx, currentDeploymentInEnvQuery, participantID, environmentID).Scan(&version, &contractID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, false
	}

	if err != nil {
		panic(fmt.Errorf("error loading current deployment in env: %w", err))
	}

	return version, contractID, true
}

func (this *DeploymentRepository) ListCurrentDeploymentsInEnv(ctx context.Context, environmentID int64) []model.Deployment {
	rows, err := this.pool.Query(ctx, listCurrentDeploymentsInEnvQuery, environmentID)
	if err != nil {
		panic(fmt.Errorf("error listing current deployments in env: %w", err))
	}

	deployments, err := pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Participant: &model.Participant{}}
		err := row.Scan(&deployment.Participant.ID, &deployment.Participant.Name, &deployment.Version, &deployment.DeployedAt)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning deployment row: %w", err))
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading current deployments in env: %w", err))
	}

	return deployments
}
