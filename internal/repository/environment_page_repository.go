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
	findEnvironmentPageEnvironmentQuery = `
		SELECT
			id, name
		FROM
			environments
		WHERE
			id = $1
	`

	countEnvironmentDeploymentsQuery = `
		SELECT
			COUNT(*)
		FROM
			deployments
		WHERE
			environment_id = $1
	`

	listEnvironmentDeploymentsQuery = `
		SELECT
			d.id, p.id, p.name, d.version, d.rollback, d.deployed_at
		FROM
			deployments d
			JOIN participants p ON p.id = d.participant_id
		WHERE
			d.environment_id = $1
		ORDER BY
			d.deployed_at DESC, d.id DESC
		LIMIT $2 OFFSET $3
	`

	countEnvironmentChecksQuery = `
		SELECT
			COUNT(*)
		FROM
			compatibility_checks
		WHERE
			environment_id = $1
	`

	listEnvironmentChecksQuery = `
		SELECT
			c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at
		FROM
			compatibility_checks c
			JOIN participants p ON p.id = c.participant_id
			JOIN environments e ON e.id = c.environment_id
		WHERE
			c.environment_id = $1
		ORDER BY
			c.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3
	`

	listEnvironmentCheckResultsQuery = `
		SELECT
			id, check_id, counterpart_name, counterpart_participant_id, counterpart_version,
			verdict_contract_id_one, verdict_contract_id_two, deployable, COALESCE(role, '')
		FROM
			compatibility_check_results
		WHERE
			check_id = ANY($1)
		ORDER BY
			check_id, counterpart_name COLLATE "C"
	`
)

type EnvironmentPageRepository struct {
	pool *pgxpool.Pool
}

func NewEnvironmentPageRepository(pool *pgxpool.Pool) *EnvironmentPageRepository {
	return &EnvironmentPageRepository{pool: pool}
}

func (this *EnvironmentPageRepository) FindEnvironment(ctx context.Context, environmentID int64) (*model.Environment, bool) {
	environment := &model.Environment{}
	err := this.pool.QueryRow(ctx, findEnvironmentPageEnvironmentQuery, environmentID).Scan(&environment.ID, &environment.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding environment: %w", err))
	}

	return environment, true
}

func (this *EnvironmentPageRepository) ListDeployments(
	ctx context.Context,
	environmentID int64,
	limit, offset int,
) (deployments []model.Deployment, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countEnvironmentDeploymentsQuery, environmentID).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting environment deployments: %w", err))
	}

	rows, err := tx.Query(ctx, listEnvironmentDeploymentsQuery, environmentID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing environment deployments: %w", err))
	}

	deployments, err = pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Participant: &model.Participant{}}
		err := row.Scan(
			&deployment.ID,
			&deployment.Participant.ID,
			&deployment.Participant.Name,
			&deployment.Version,
			&deployment.Rollback,
			&deployment.DeployedAt,
		)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning environment deployments: %w", err))
	}

	return deployments, total
}

func (this *EnvironmentPageRepository) ListChecks(
	ctx context.Context,
	environmentID int64,
	limit, offset int,
) (checks []model.CompatibilityCheck, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countEnvironmentChecksQuery, environmentID).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting environment checks: %w", err))
	}

	rows, err := tx.Query(ctx, listEnvironmentChecksQuery, environmentID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing environment checks: %w", err))
	}

	checks, err = pgx.AppendRows(make([]model.CompatibilityCheck, 0), rows, func(row pgx.CollectableRow) (model.CompatibilityCheck, error) {
		var check model.CompatibilityCheck
		err := row.Scan(
			&check.ID,
			&check.ParticipantID,
			&check.ParticipantName,
			&check.ContractID,
			&check.Version,
			&check.EnvironmentID,
			&check.EnvironmentName,
			&check.Deployable,
			&check.Report,
			&check.CreatedAt,
		)

		return check, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning environment checks: %w", err))
	}

	return checks, total
}

func (this *EnvironmentPageRepository) ListResultsByCheck(ctx context.Context, checkIDs []int64) map[int64][]model.CompatibilityCheckResult {
	rows, err := this.pool.Query(ctx, listEnvironmentCheckResultsQuery, checkIDs)
	if err != nil {
		panic(fmt.Errorf("error listing environment check results: %w", err))
	}

	results, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.CompatibilityCheckResult, error) {
		var result model.CompatibilityCheckResult
		err := row.Scan(
			&result.ID,
			&result.CheckID,
			&result.CounterpartName,
			&result.CounterpartParticipantID,
			&result.CounterpartVersion,
			&result.VerdictContractIDOne,
			&result.VerdictContractIDTwo,
			&result.Deployable,
			&result.Role,
		)

		return result, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning environment check results: %w", err))
	}

	resultsByCheck := make(map[int64][]model.CompatibilityCheckResult, len(checkIDs))
	for _, result := range results {
		resultsByCheck[result.CheckID] = append(resultsByCheck[result.CheckID], result)
	}

	return resultsByCheck
}
