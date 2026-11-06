package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
)

const (
	findContractVersionQuery = `
		SELECT
			cv.id, p.id, p.name, cv.version, cv.contract_content, cv.created_at
		FROM
			contract_versions cv
			JOIN participants p ON p.id = cv.participant_id
		WHERE
			p.name = $1 AND cv.version = $2
	`

	listCurrentDeploymentsOfVersionQuery = `
		SELECT
			e.id, e.name, cd.deployed_at
		FROM
			current_deployments cd
			JOIN environments e ON e.id = cd.environment_id
		WHERE
			cd.participant_id = $1 AND cd.version = $2
		ORDER BY
			e.name COLLATE "C"
	`

	countDeploymentsOfVersionQuery = `
		SELECT
			count(*)
		FROM
			deployments
		WHERE
			participant_id = $1 AND version = $2
	`

	listDeploymentsOfVersionQuery = `
		SELECT
			d.id, e.id, e.name, d.rollback, d.deployed_at
		FROM
			deployments d
			JOIN environments e ON e.id = d.environment_id
		WHERE
			d.participant_id = $1 AND d.version = $2
		ORDER BY
			d.deployed_at DESC, d.id DESC
		LIMIT $3 OFFSET $4
	`

	listLatestChecksOfVersionQuery = `
		SELECT * FROM (
			SELECT DISTINCT ON (c.environment_id)
				c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
				COALESCE(c.environment_name, e.name) AS environment_name, c.deployable, c.report, c.created_at
			FROM
				compatibility_checks c
				JOIN participants p ON p.id = c.participant_id
				JOIN environments e ON e.id = c.environment_id
			WHERE
				c.participant_id = $1 AND c.version = $2
			ORDER BY
				c.environment_id, c.created_at DESC, c.id DESC
		) latest
		ORDER BY
			environment_name COLLATE "C"
	`

	countChecksOfVersionQuery = `
		SELECT
			count(*)
		FROM
			compatibility_checks
		WHERE
			participant_id = $1 AND version = $2
	`

	listChecksOfVersionQuery = `
		SELECT
			c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at
		FROM
			compatibility_checks c
			JOIN participants p ON p.id = c.participant_id
			JOIN environments e ON e.id = c.environment_id
		WHERE
			c.participant_id = $1 AND c.version = $2
		ORDER BY
			c.created_at DESC, c.id DESC
		LIMIT $3 OFFSET $4
	`

	listResultsOfChecksQuery = `
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

func (this *ContractRepository) FindContractVersion(ctx context.Context, participantName, version string) (*model.ContractVersion, bool) {
	contractVersion := &model.ContractVersion{}

	err := this.pool.QueryRow(ctx, findContractVersionQuery, participantName, version).Scan(
		&contractVersion.ID,
		&contractVersion.ParticipantID,
		&contractVersion.ParticipantName,
		&contractVersion.Version,
		&contractVersion.ContractContent,
		&contractVersion.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding contract version: %w", err))
	}

	return contractVersion, true
}

func (this *DeploymentRepository) ListCurrentDeploymentsOfVersion(ctx context.Context, participantID int64, version string) []model.Deployment {
	rows, err := this.pool.Query(ctx, listCurrentDeploymentsOfVersionQuery, participantID, version)
	if err != nil {
		panic(fmt.Errorf("error listing current deployments of version: %w", err))
	}

	deployments, err := pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Version: version, Environment: &model.Environment{}}
		err := row.Scan(&deployment.Environment.ID, &deployment.Environment.Name, &deployment.DeployedAt)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning current deployments of version: %w", err))
	}

	return deployments
}

func (this *DeploymentRepository) ListDeploymentsOfVersion(
	ctx context.Context,
	participantID int64,
	version string,
	limit, offset int,
) (deployments []model.Deployment, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countDeploymentsOfVersionQuery, participantID, version).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting deployments of version: %w", err))
	}

	rows, err := tx.Query(ctx, listDeploymentsOfVersionQuery, participantID, version, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing deployments of version: %w", err))
	}

	deployments, err = pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Version: version, Environment: &model.Environment{}}
		err := row.Scan(&deployment.ID, &deployment.Environment.ID, &deployment.Environment.Name, &deployment.Rollback, &deployment.DeployedAt)

		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning deployments of version: %w", err))
	}

	return deployments, total
}

func (this *CompatibilityRepository) ListLatestChecksOfVersion(ctx context.Context, participantID int64, version string) []model.CompatibilityCheck {
	rows, err := this.pool.Query(ctx, listLatestChecksOfVersionQuery, participantID, version)
	if err != nil {
		panic(fmt.Errorf("error listing latest checks of version: %w", err))
	}

	return collectCompatibilityChecks(rows)
}

func (this *CompatibilityRepository) ListChecksOfVersion(
	ctx context.Context,
	participantID int64,
	version string,
	limit, offset int,
) (checks []model.CompatibilityCheck, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countChecksOfVersionQuery, participantID, version).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting checks of version: %w", err))
	}

	rows, err := tx.Query(ctx, listChecksOfVersionQuery, participantID, version, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing checks of version: %w", err))
	}

	return collectCompatibilityChecks(rows), total
}

func (this *CompatibilityRepository) ListResultsOfChecks(ctx context.Context, checkIDs []int64) map[int64][]model.CompatibilityCheckResult {
	rows, err := this.pool.Query(ctx, listResultsOfChecksQuery, checkIDs)
	if err != nil {
		panic(fmt.Errorf("error listing results of checks: %w", err))
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
		panic(fmt.Errorf("error scanning results of checks: %w", err))
	}

	resultsByCheck := make(map[int64][]model.CompatibilityCheckResult, len(checkIDs))
	for _, result := range results {
		resultsByCheck[result.CheckID] = append(resultsByCheck[result.CheckID], result)
	}

	return resultsByCheck
}

func collectCompatibilityChecks(rows pgx.Rows) []model.CompatibilityCheck {
	checks, err := pgx.AppendRows(make([]model.CompatibilityCheck, 0), rows, func(row pgx.CollectableRow) (model.CompatibilityCheck, error) {
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
		panic(fmt.Errorf("error scanning compatibility checks: %w", err))
	}

	return checks
}
