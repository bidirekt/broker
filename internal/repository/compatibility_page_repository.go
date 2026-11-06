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
	findCompatibilityParticipantQuery = `
		SELECT
			id, name
		FROM
			participants
		WHERE
			name = $1
	`

	listCompatibilityEnvironmentsQuery = `
		SELECT
			id, name
		FROM
			environments
		ORDER BY
			name COLLATE "C"
	`

	countCompatibilityVersionsQuery = `
		SELECT
			COUNT(*)
		FROM
			contract_versions
		WHERE
			participant_id = $1
	`

	listCompatibilityVersionsQuery = `
		SELECT
			cv.version,
			cv.created_at,
			COALESCE(array_agg(e.id ORDER BY e.name COLLATE "C") FILTER (WHERE e.id IS NOT NULL), '{}'),
			COALESCE(array_agg(e.name ORDER BY e.name COLLATE "C") FILTER (WHERE e.id IS NOT NULL), '{}')
		FROM
			contract_versions cv
		LEFT JOIN
			current_deployments cd ON cd.participant_id = cv.participant_id AND cd.version = cv.version
		LEFT JOIN
			environments e ON e.id = cd.environment_id
		WHERE
			cv.participant_id = $1
		GROUP BY
			cv.id
		ORDER BY
			cv.id DESC
		LIMIT $2 OFFSET $3
	`

	listCompatibilityLatestChecksQuery = `
		SELECT DISTINCT ON (c.version, c.environment_id)
			c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at
		FROM
			compatibility_checks c
			JOIN participants p ON p.id = c.participant_id
			JOIN environments e ON e.id = c.environment_id
		WHERE
			c.participant_id = $1
		AND
			c.version = ANY($2)
		ORDER BY
			c.version, c.environment_id, c.created_at DESC, c.id DESC
	`

	listCompatibilityPageCheckResultsQuery = `
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

	listCompatibilityDeploymentsSinceChecksQuery = `
		SELECT
			c.id,
			COUNT(*),
			array_agg(DISTINCT p.name COLLATE "C" ORDER BY p.name COLLATE "C")
		FROM
			compatibility_checks c
			JOIN deployments d ON d.environment_id = c.environment_id
				AND d.deployed_at > c.created_at
				AND d.participant_id <> c.participant_id
			JOIN participants p ON p.id = d.participant_id
		WHERE
			c.id = ANY($1)
		GROUP BY
			c.id
	`
)

type CompatibilityPageRepository struct {
	pool *pgxpool.Pool
}

func NewCompatibilityPageRepository(pool *pgxpool.Pool) *CompatibilityPageRepository {
	return &CompatibilityPageRepository{pool: pool}
}

func (this *CompatibilityPageRepository) FindParticipant(ctx context.Context, name string) (*model.Participant, bool) {
	participant := &model.Participant{}
	err := this.pool.QueryRow(ctx, findCompatibilityParticipantQuery, name).Scan(&participant.ID, &participant.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding compatibility participant: %w", err))
	}

	return participant, true
}

func (this *CompatibilityPageRepository) ListEnvironments(ctx context.Context) []model.Environment {
	rows, err := this.pool.Query(ctx, listCompatibilityEnvironmentsQuery)
	if err != nil {
		panic(fmt.Errorf("error listing compatibility environments: %w", err))
	}

	environments, err := pgx.AppendRows(make([]model.Environment, 0), rows, func(row pgx.CollectableRow) (model.Environment, error) {
		var environment model.Environment
		err := row.Scan(&environment.ID, &environment.Name)
		return environment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning compatibility environments: %w", err))
	}

	return environments
}

func (this *CompatibilityPageRepository) ListVersions(
	ctx context.Context,
	participantID int64,
	limit, offset int,
) (versions []model.PublishedVersion, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countCompatibilityVersionsQuery, participantID).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting compatibility versions: %w", err))
	}

	rows, err := tx.Query(ctx, listCompatibilityVersionsQuery, participantID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing compatibility versions: %w", err))
	}

	versions, err = pgx.AppendRows(make([]model.PublishedVersion, 0), rows, func(row pgx.CollectableRow) (model.PublishedVersion, error) {
		var (
			version          model.PublishedVersion
			environmentIDs   []int64
			environmentNames []string
		)
		if err := row.Scan(&version.Version, &version.PublishedAt, &environmentIDs, &environmentNames); err != nil {
			return version, err
		}

		version.DeployedIn = make([]model.Environment, len(environmentIDs))
		for i := range environmentIDs {
			version.DeployedIn[i] = model.Environment{ID: environmentIDs[i], Name: environmentNames[i]}
		}

		return version, nil
	})
	if err != nil {
		panic(fmt.Errorf("error scanning compatibility versions: %w", err))
	}

	return versions, total
}

func (this *CompatibilityPageRepository) ListLatestChecks(ctx context.Context, participantID int64, versions []string) []model.CompatibilityCheck {
	rows, err := this.pool.Query(ctx, listCompatibilityLatestChecksQuery, participantID, versions)
	if err != nil {
		panic(fmt.Errorf("error listing compatibility latest checks: %w", err))
	}

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
		panic(fmt.Errorf("error scanning compatibility latest checks: %w", err))
	}

	return checks
}

func (this *CompatibilityPageRepository) ListResultsByCheck(ctx context.Context, checkIDs []int64) map[int64][]model.CompatibilityCheckResult {
	rows, err := this.pool.Query(ctx, listCompatibilityPageCheckResultsQuery, checkIDs)
	if err != nil {
		panic(fmt.Errorf("error listing compatibility check results: %w", err))
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
		panic(fmt.Errorf("error scanning compatibility check results: %w", err))
	}

	resultsByCheck := make(map[int64][]model.CompatibilityCheckResult, len(checkIDs))
	for _, result := range results {
		resultsByCheck[result.CheckID] = append(resultsByCheck[result.CheckID], result)
	}

	return resultsByCheck
}

func (this *CompatibilityPageRepository) ListDeploymentsSinceChecks(ctx context.Context, checkIDs []int64) map[int64]model.DeploymentsSinceCheck {
	rows, err := this.pool.Query(ctx, listCompatibilityDeploymentsSinceChecksQuery, checkIDs)
	if err != nil {
		panic(fmt.Errorf("error listing deployments since checks: %w", err))
	}

	defer rows.Close()

	deploymentsByCheck := make(map[int64]model.DeploymentsSinceCheck, len(checkIDs))
	for rows.Next() {
		var (
			checkID int64
			since   model.DeploymentsSinceCheck
		)
		if err := rows.Scan(&checkID, &since.Count, &since.Participants); err != nil {
			panic(fmt.Errorf("error scanning deployments since checks: %w", err))
		}
		deploymentsByCheck[checkID] = since
	}
	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error scanning deployments since checks: %w", err))
	}

	return deploymentsByCheck
}
