package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	findParticipantDeploymentsHeaderQuery = `
		SELECT
			p.id,
			p.name,
			lcv.version,
			(SELECT COUNT(*) FROM contract_versions WHERE participant_id = p.id),
			(SELECT COUNT(*) FROM deployments WHERE participant_id = p.id),
			(SELECT COUNT(*) FROM compatibility_checks WHERE participant_id = p.id),
			(SELECT COUNT(*) FROM (
				SELECT r.consumed_provider AS name
				FROM resources r
				JOIN LATERAL (
					SELECT change_type
					FROM resource_versions
					WHERE resource_id = r.id AND contract_id <= lcv.contract_id
					ORDER BY contract_id DESC
					LIMIT 1
				) rv ON true
				WHERE r.participant_id = p.id
				  AND r.direction = 'consumes'
				  AND rv.change_type = 'added'
				UNION
				SELECT consumer.name
				FROM resources r
				JOIN current_deployments cd ON cd.participant_id = r.participant_id
				JOIN participants consumer ON consumer.id = r.participant_id
				JOIN LATERAL (
					SELECT change_type
					FROM resource_versions
					WHERE resource_id = r.id AND contract_id <= cd.contract_id
					ORDER BY contract_id DESC
					LIMIT 1
				) rv ON true
				WHERE r.direction = 'consumes'
				  AND r.consumed_provider = p.name
				  AND rv.change_type = 'added'
			) relations)
		FROM participants p
		LEFT JOIN latest_contract_versions lcv ON lcv.participant_id = p.id
		WHERE p.name = $1
	`

	listParticipantCurrentDeploymentsQuery = `
		SELECT e.id, e.name, cd.version, cd.deployed_at
		FROM current_deployments cd
		JOIN environments e ON e.id = cd.environment_id
		WHERE cd.participant_id = $1
		ORDER BY e.name COLLATE "C"
	`

	listParticipantDeployedEnvironmentsQuery = `
		SELECT e.id, e.name
		FROM environments e
		WHERE EXISTS (SELECT 1 FROM deployments d WHERE d.participant_id = $1 AND d.environment_id = e.id)
		ORDER BY e.name COLLATE "C"
	`

	countParticipantDeploymentsQuery = `
		SELECT COUNT(*)
		FROM deployments
		WHERE participant_id = $1 AND ($2::bigint IS NULL OR environment_id = $2)
	`

	listParticipantDeploymentsQuery = `
		SELECT d.id, d.version, e.id, e.name, d.rollback, d.deployed_at
		FROM deployments d
		JOIN environments e ON e.id = d.environment_id
		WHERE d.participant_id = $1 AND ($2::bigint IS NULL OR d.environment_id = $2)
		ORDER BY d.deployed_at DESC, d.id DESC
		LIMIT $3 OFFSET $4
	`
)

type ParticipantDeploymentsHeader struct {
	Participant     *model.Participant
	LatestVersion   null.String
	Deployed        []model.Deployment
	VersionCount    int
	DeploymentCount int
	CheckCount      int
	RelationCount   int
}

type ParticipantDeploymentsRepository struct {
	pool *pgxpool.Pool
}

func NewParticipantDeploymentsRepository(pool *pgxpool.Pool) *ParticipantDeploymentsRepository {
	return &ParticipantDeploymentsRepository{pool: pool}
}

func (this *ParticipantDeploymentsRepository) FindHeader(ctx context.Context, name string) (*ParticipantDeploymentsHeader, bool) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	header := &ParticipantDeploymentsHeader{Participant: &model.Participant{}}
	err = tx.QueryRow(ctx, findParticipantDeploymentsHeaderQuery, name).Scan(
		&header.Participant.ID,
		&header.Participant.Name,
		&header.LatestVersion,
		&header.VersionCount,
		&header.DeploymentCount,
		&header.CheckCount,
		&header.RelationCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding participant header: %w", err))
	}

	rows, err := tx.Query(ctx, listParticipantCurrentDeploymentsQuery, header.Participant.ID)
	if err != nil {
		panic(fmt.Errorf("error listing participant current deployments: %w", err))
	}

	header.Deployed, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Environment: &model.Environment{}}
		err := row.Scan(&deployment.Environment.ID, &deployment.Environment.Name, &deployment.Version, &deployment.DeployedAt)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing participant current deployments: %w", err))
	}

	return header, true
}

func (this *ParticipantDeploymentsRepository) ListDeployedEnvironments(ctx context.Context, participantID int64) []*model.Environment {
	rows, err := this.pool.Query(ctx, listParticipantDeployedEnvironmentsQuery, participantID)
	if err != nil {
		panic(fmt.Errorf("error listing participant deployed environments: %w", err))
	}

	environments, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.Environment, error) {
		environment := &model.Environment{}
		err := row.Scan(&environment.ID, &environment.Name)

		return environment, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing participant deployed environments: %w", err))
	}

	return environments
}

func (this *ParticipantDeploymentsRepository) FindEnvironment(ctx context.Context, environmentID int64) (*model.Environment, bool) {
	environment := &model.Environment{}
	err := this.pool.QueryRow(ctx, findEnvironmentByIDQuery, environmentID).Scan(&environment.ID, &environment.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding environment: %w", err))
	}

	return environment, true
}

func (this *ParticipantDeploymentsRepository) List(
	ctx context.Context,
	participantID int64,
	environmentID null.Int,
	limit, offset int,
) (deployments []model.Deployment, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countParticipantDeploymentsQuery, participantID, environmentID).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting participant deployments: %w", err))
	}

	rows, err := tx.Query(ctx, listParticipantDeploymentsQuery, participantID, environmentID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing participant deployments: %w", err))
	}

	deployments, err = pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Environment: &model.Environment{}}
		err := row.Scan(
			&deployment.ID,
			&deployment.Version,
			&deployment.Environment.ID,
			&deployment.Environment.Name,
			&deployment.Rollback,
			&deployment.DeployedAt,
		)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing participant deployments: %w", err))
	}

	return deployments, total
}
