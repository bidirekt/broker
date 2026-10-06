package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertDeploymentQuery = `
		WITH prior AS (
			SELECT version, deployed_at
			FROM deployments
			WHERE participant_id = $1 AND environment_id = $3
		),
		latest AS (
			SELECT version
			FROM prior
			ORDER BY deployed_at DESC
			LIMIT 1
		)
		INSERT INTO deployments
			(participant_id, version, environment_id, rollback)
		SELECT
			$1,
			$2,
			$3,
			EXISTS (SELECT 1 FROM prior WHERE version = $2)
		WHERE COALESCE((SELECT version FROM latest), '') <> $2
		RETURNING id, rollback, deployed_at
	`

	currentVersionInEnvQuery = `
		SELECT DISTINCT ON (participant_id)
			version
		FROM
			deployments
		WHERE
			participant_id = $1 AND environment_id = $2
		ORDER BY
			participant_id, deployed_at DESC
		LIMIT 1
	`

	listCurrentDeploymentsInEnvQuery = `
		SELECT DISTINCT ON (d.participant_id)
			p.id, p.name, d.version, d.deployed_at
		FROM deployments d
		JOIN participants p ON p.id = d.participant_id
		WHERE d.environment_id = $1
		ORDER BY d.participant_id, d.deployed_at DESC
	`
)

type DeploymentRepository struct {
	pool *pgxpool.Pool
}

func NewDeploymentRepository(pool *pgxpool.Pool) *DeploymentRepository {
	return &DeploymentRepository{pool: pool}
}

func (this *DeploymentRepository) Insert(ctx context.Context, d *model.Deployment) {
	err := this.pool.QueryRow(
		ctx,
		insertDeploymentQuery,
		d.Participant.ID,
		d.Version,
		d.Environment.ID,
	).Scan(&d.ID, &d.Rollback, &d.DeployedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		panic(fmt.Errorf("error inserting deployment: %w", err))
	}
}

func (this *DeploymentRepository) CurrentVersionInEnv(ctx context.Context, participantID int64, environmentID int64) (string, bool) {
	var version string
	err := this.pool.QueryRow(ctx, currentVersionInEnvQuery, participantID, environmentID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false
	}
	if err != nil {
		panic(fmt.Errorf("error loading current version in env: %w", err))
	}
	return version, true
}

func (this *DeploymentRepository) ListCurrentDeploymentsInEnv(ctx context.Context, environmentID int64) []model.Deployment {
	rows, err := this.pool.Query(ctx, listCurrentDeploymentsInEnvQuery, environmentID)
	if err != nil {
		panic(fmt.Errorf("error listing current deployments in env: %w", err))
	}
	defer rows.Close()

	var deployments []model.Deployment
	for rows.Next() {
		var (
			participantID   int64
			participantName string
			version         string
			deployedAt      time.Time
		)
		if err := rows.Scan(&participantID, &participantName, &version, &deployedAt); err != nil {
			panic(fmt.Errorf("error scanning deployment row: %w", err))
		}
		deployments = append(deployments, model.Deployment{
			Participant: &model.Participant{ID: participantID, Name: participantName},
			Version:     version,
			DeployedAt:  deployedAt,
		})
	}
	return deployments
}
