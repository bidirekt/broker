package repository

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	countOverviewNumbersQuery = `
		SELECT
			(SELECT COUNT(*) FROM participants),
			(SELECT COUNT(*) FROM participants p LEFT JOIN latest_contract_versions l ON l.participant_id = p.id WHERE l.participant_id IS NULL),
			(SELECT COUNT(*) FROM environments),
			(SELECT COUNT(*) FROM contract_versions),
			(SELECT COUNT(*) FROM compatibility_checks WHERE created_at >= date_trunc('month', now(), 'UTC'))
	`

	listOverviewSaidNoChecksQuery = `
		SELECT
			c.id, p.id, p.name, c.contract_id, c.version, e.id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at,
			EXISTS (
				SELECT 1
				FROM deployments d
				WHERE d.participant_id = p.id
					AND d.environment_id = e.id
					AND d.version = c.version
					AND d.deployed_at > c.created_at
			),
			COUNT(*) OVER ()
		FROM
			participants p
			CROSS JOIN environments e
			CROSS JOIN LATERAL (
				SELECT
					id, contract_id, version, environment_name, deployable, report, created_at
				FROM
					compatibility_checks
				WHERE
					participant_id = p.id
					AND environment_id = e.id
				ORDER BY
					created_at DESC, id DESC
				LIMIT 1
			) c
		WHERE
			NOT c.deployable
		ORDER BY
			c.created_at DESC, c.id DESC
		LIMIT $1
	`

	listOverviewEnvironmentsQuery = `
		SELECT
			id, name
		FROM
			environments
		ORDER BY
			name COLLATE "C"
	`

	listOverviewCurrentDeploymentsQuery = `
		WITH shown AS (
			SELECT
				c.participant_id, p.name, MAX(c.deployed_at) AS last_deployed_at, COUNT(*) OVER () AS total
			FROM
				current_deployments c
				JOIN participants p ON p.id = c.participant_id
			GROUP BY
				c.participant_id, p.name
			ORDER BY
				last_deployed_at DESC, p.name COLLATE "C"
			LIMIT $1
		)
		SELECT
			s.participant_id, s.name, e.id, e.name, c.version, c.deployed_at, s.total
		FROM
			shown s
			JOIN current_deployments c ON c.participant_id = s.participant_id
			JOIN environments e ON e.id = c.environment_id
		ORDER BY
			s.last_deployed_at DESC, s.name COLLATE "C", e.name COLLATE "C"
	`

	listOverviewRecentDeploymentsQuery = `
		SELECT
			d.id, p.id, p.name, d.version, e.id, e.name, d.rollback, d.deployed_at
		FROM
			deployments d
			JOIN participants p ON p.id = d.participant_id
			JOIN environments e ON e.id = d.environment_id
		ORDER BY
			d.deployed_at DESC, d.id DESC
		LIMIT $1
	`

	listOverviewRecentChecksQuery = `
		SELECT
			c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at
		FROM
			compatibility_checks c
			JOIN participants p ON p.id = c.participant_id
			JOIN environments e ON e.id = c.environment_id
		ORDER BY
			c.created_at DESC, c.id DESC
		LIMIT $1
	`

	listOverviewCheckResultsQuery = `
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

type OverviewRepository struct {
	pool *pgxpool.Pool
}

func NewOverviewRepository(pool *pgxpool.Pool) *OverviewRepository {
	return &OverviewRepository{pool: pool}
}

func (this *OverviewRepository) CountNumbers(ctx context.Context) model.OverviewNumbers {
	var numbers model.OverviewNumbers
	err := this.pool.QueryRow(ctx, countOverviewNumbersQuery).Scan(
		&numbers.Participants,
		&numbers.NeverPublished,
		&numbers.Environments,
		&numbers.VersionsPublished,
		&numbers.ChecksThisMonth,
	)
	if err != nil {
		panic(fmt.Errorf("error counting overview numbers: %w", err))
	}

	return numbers
}

func (this *OverviewRepository) ListSaidNoChecks(ctx context.Context, limit int) (checks []model.OverviewSaidNoCheck, total int) {
	rows, err := this.pool.Query(ctx, listOverviewSaidNoChecksQuery, limit)
	if err != nil {
		panic(fmt.Errorf("error listing checks that said no: %w", err))
	}

	checks, err = pgx.AppendRows(make([]model.OverviewSaidNoCheck, 0), rows, func(row pgx.CollectableRow) (model.OverviewSaidNoCheck, error) {
		var check model.OverviewSaidNoCheck
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
			&check.DeployedThereAnyway,
			&total,
		)

		return check, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning checks that said no: %w", err))
	}

	return checks, total
}

func (this *OverviewRepository) ListEnvironments(ctx context.Context) []model.Environment {
	rows, err := this.pool.Query(ctx, listOverviewEnvironmentsQuery)
	if err != nil {
		panic(fmt.Errorf("error listing overview environments: %w", err))
	}

	environments, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Environment, error) {
		var environment model.Environment
		err := row.Scan(&environment.ID, &environment.Name)

		return environment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning overview environments: %w", err))
	}

	return environments
}

func (this *OverviewRepository) ListCurrentDeployments(ctx context.Context, participantLimit int) (deployments []model.Deployment, participantsTotal int) {
	rows, err := this.pool.Query(ctx, listOverviewCurrentDeploymentsQuery, participantLimit)
	if err != nil {
		panic(fmt.Errorf("error listing current deployments: %w", err))
	}

	deployments, err = pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Participant: &model.Participant{}, Environment: &model.Environment{}}
		err := row.Scan(
			&deployment.Participant.ID,
			&deployment.Participant.Name,
			&deployment.Environment.ID,
			&deployment.Environment.Name,
			&deployment.Version,
			&deployment.DeployedAt,
			&participantsTotal,
		)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning current deployments: %w", err))
	}

	return deployments, participantsTotal
}

func (this *OverviewRepository) ListRecentDeployments(ctx context.Context, limit int) []model.Deployment {
	rows, err := this.pool.Query(ctx, listOverviewRecentDeploymentsQuery, limit)
	if err != nil {
		panic(fmt.Errorf("error listing recent deployments: %w", err))
	}

	deployments, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Participant: &model.Participant{}, Environment: &model.Environment{}}
		err := row.Scan(
			&deployment.ID,
			&deployment.Participant.ID,
			&deployment.Participant.Name,
			&deployment.Version,
			&deployment.Environment.ID,
			&deployment.Environment.Name,
			&deployment.Rollback,
			&deployment.DeployedAt,
		)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning recent deployments: %w", err))
	}

	return deployments
}

func (this *OverviewRepository) ListRecentChecks(ctx context.Context, limit int) []model.CompatibilityCheck {
	rows, err := this.pool.Query(ctx, listOverviewRecentChecksQuery, limit)
	if err != nil {
		panic(fmt.Errorf("error listing recent checks: %w", err))
	}

	checks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.CompatibilityCheck, error) {
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
		panic(fmt.Errorf("error scanning recent checks: %w", err))
	}

	return checks
}

func (this *OverviewRepository) ListResultsByCheck(ctx context.Context, checkIDs []int64) map[int64][]model.CompatibilityCheckResult {
	rows, err := this.pool.Query(ctx, listOverviewCheckResultsQuery, checkIDs)
	if err != nil {
		panic(fmt.Errorf("error listing overview check results: %w", err))
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
		panic(fmt.Errorf("error scanning overview check results: %w", err))
	}

	resultsByCheck := make(map[int64][]model.CompatibilityCheckResult, len(checkIDs))
	for _, result := range results {
		resultsByCheck[result.CheckID] = append(resultsByCheck[result.CheckID], result)
	}

	return resultsByCheck
}
