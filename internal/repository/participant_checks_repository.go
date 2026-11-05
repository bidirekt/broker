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
	findParticipantChecksHeaderQuery = `
		SELECT
			p.id,
			p.name,
			lcv.version,
			(SELECT count(*) FROM contract_versions WHERE participant_id = p.id),
			(SELECT count(*) FROM deployments WHERE participant_id = p.id),
			(SELECT count(*) FROM compatibility_checks WHERE participant_id = p.id),
			(SELECT count(*) FROM (
				SELECT
					r.consumed_provider
				FROM
					resources r
					JOIN LATERAL (
						SELECT change_type
						FROM resource_versions
						WHERE resource_id = r.id AND contract_id <= lcv.contract_id
						ORDER BY contract_id DESC
						LIMIT 1
					) rv ON true
				WHERE
					r.participant_id = p.id
					AND r.direction = 'consumes'
					AND rv.change_type = 'added'
				UNION
				SELECT
					consumer.name
				FROM
					current_deployments cd
					JOIN participants consumer ON consumer.id = cd.participant_id
					JOIN resources r ON r.participant_id = cd.participant_id
					JOIN LATERAL (
						SELECT change_type
						FROM resource_versions
						WHERE resource_id = r.id AND contract_id <= cd.contract_id
						ORDER BY contract_id DESC
						LIMIT 1
					) rv ON true
				WHERE
					r.direction = 'consumes'
					AND r.consumed_provider = p.name
					AND rv.change_type = 'added'
			) relations)
		FROM
			participants p
			LEFT JOIN latest_contract_versions lcv ON lcv.participant_id = p.id
		WHERE
			p.name = $1
	`

	listParticipantChecksDeployedQuery = `
		SELECT
			e.id, e.name, cd.version
		FROM
			current_deployments cd
			JOIN environments e ON e.id = cd.environment_id
		WHERE
			cd.participant_id = $1
		ORDER BY
			e.name COLLATE "C"
	`

	listParticipantChecksQuery = `
		SELECT
			c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at
		FROM
			compatibility_checks c
			JOIN participants p ON p.id = c.participant_id
			JOIN environments e ON e.id = c.environment_id
		WHERE
			c.participant_id = $1
		ORDER BY
			c.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3
	`

	listParticipantCheckResultsQuery = `
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

type ParticipantChecksHeader struct {
	Participant   model.Participant
	LatestVersion null.String
	Deployed      []model.Deployment
	Versions      int
	Deployments   int
	Checks        int
	Relations     int
}

type ParticipantChecksRepository struct {
	pool *pgxpool.Pool
}

func NewParticipantChecksRepository(pool *pgxpool.Pool) *ParticipantChecksRepository {
	return &ParticipantChecksRepository{pool: pool}
}

func (this *ParticipantChecksRepository) FindHeader(ctx context.Context, name string) (*ParticipantChecksHeader, bool) {
	header := &ParticipantChecksHeader{}

	err := this.pool.QueryRow(ctx, findParticipantChecksHeaderQuery, name).Scan(
		&header.Participant.ID,
		&header.Participant.Name,
		&header.LatestVersion,
		&header.Versions,
		&header.Deployments,
		&header.Checks,
		&header.Relations,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding participant checks header: %w", err))
	}

	rows, err := this.pool.Query(ctx, listParticipantChecksDeployedQuery, header.Participant.ID)
	if err != nil {
		panic(fmt.Errorf("error listing participant deployments now: %w", err))
	}

	header.Deployed, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Environment: &model.Environment{}}
		err := row.Scan(&deployment.Environment.ID, &deployment.Environment.Name, &deployment.Version)
		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning participant deployments now: %w", err))
	}

	return header, true
}

func (this *ParticipantChecksRepository) ListChecks(ctx context.Context, participantID int64, limit, offset int) []model.CompatibilityCheck {
	rows, err := this.pool.Query(ctx, listParticipantChecksQuery, participantID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing participant checks: %w", err))
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
		panic(fmt.Errorf("error scanning participant checks: %w", err))
	}

	return checks
}

func (this *ParticipantChecksRepository) ListResultsByCheck(ctx context.Context, checkIDs []int64) map[int64][]model.CompatibilityCheckResult {
	rows, err := this.pool.Query(ctx, listParticipantCheckResultsQuery, checkIDs)
	if err != nil {
		panic(fmt.Errorf("error listing participant check results: %w", err))
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
		panic(fmt.Errorf("error scanning participant check results: %w", err))
	}

	resultsByCheck := make(map[int64][]model.CompatibilityCheckResult, len(checkIDs))
	for _, result := range results {
		resultsByCheck[result.CheckID] = append(resultsByCheck[result.CheckID], result)
	}

	return resultsByCheck
}
