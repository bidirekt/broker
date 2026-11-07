package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertCompatibilityCheckQuery = `
		INSERT INTO compatibility_checks
			(participant_id, contract_id, version, environment_id, environment_name, deployable, report)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	insertCompatibilityCheckResultQuery = `
		INSERT INTO compatibility_check_results
			(check_id, counterpart_name, counterpart_participant_id, counterpart_version,
			 verdict_contract_id_one, verdict_contract_id_two, deployable, role)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	insertCompatibilityVerdictQuery = `
		INSERT INTO compatibility_verdicts
			(contract_id_one, contract_id_two, breaks)
		VALUES
			($1, $2, $3)
		ON CONFLICT DO NOTHING
	`

	findCompatibilityCheckQuery = `
		SELECT
			c.id, c.participant_id, p.name, c.contract_id, c.version, c.environment_id,
			COALESCE(c.environment_name, e.name), c.deployable, c.report, c.created_at
		FROM
			compatibility_checks c
			JOIN participants p ON p.id = c.participant_id
			JOIN environments e ON e.id = c.environment_id
		WHERE
			c.id = $1
	`

	listCompatibilityCheckResultsQuery = `
		SELECT
			id, check_id, counterpart_name, counterpart_participant_id, counterpart_version,
			verdict_contract_id_one, verdict_contract_id_two, deployable, COALESCE(role, '')
		FROM
			compatibility_check_results
		WHERE
			check_id = $1
		ORDER BY
			counterpart_name COLLATE "C"
	`

	findCompatibilityVerdictQuery = `
		SELECT
			contract_id_one, contract_id_two, breaks, deployable, created_at
		FROM
			compatibility_verdicts
		WHERE
			contract_id_one = $1 AND contract_id_two = $2
	`
)

type CompatibilityRepository struct {
	pool *pgxpool.Pool
}

func NewCompatibilityRepository(pool *pgxpool.Pool) *CompatibilityRepository {
	return &CompatibilityRepository{pool: pool}
}

func (this *CompatibilityRepository) RecordCheck(
	ctx context.Context,
	check *model.CompatibilityCheck,
	results []model.CompatibilityCheckResult,
	newVerdicts []model.CompatibilityVerdict,
) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	this.insertCheck(ctx, tx, check)

	for i := range newVerdicts {
		this.insertVerdict(ctx, tx, &newVerdicts[i])
	}

	for i := range results {
		results[i].CheckID = check.ID
		this.insertCheckResult(ctx, tx, &results[i])
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}
}

func (this *CompatibilityRepository) FindCheck(ctx context.Context, checkID int64) (*model.CompatibilityCheck, bool) {
	check := &model.CompatibilityCheck{}

	err := this.pool.QueryRow(ctx, findCompatibilityCheckQuery, checkID).Scan(
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
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding compatibility check: %w", err))
	}

	return check, true
}

func (this *CompatibilityRepository) ListCheckResults(ctx context.Context, checkID int64) []model.CompatibilityCheckResult {
	rows, err := this.pool.Query(ctx, listCompatibilityCheckResultsQuery, checkID)
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

	return results
}

func (this *CompatibilityRepository) GetVerdict(
	ctx context.Context,
	one, two int64,
) (*model.CompatibilityVerdict, bool) {
	contractIDOne, contractIDTwo := model.OrderContractPair(one, two)

	verdict := &model.CompatibilityVerdict{}
	var breaks []byte

	err := this.pool.QueryRow(ctx, findCompatibilityVerdictQuery, contractIDOne, contractIDTwo).Scan(
		&verdict.ContractIDOne,
		&verdict.ContractIDTwo,
		&breaks,
		&verdict.Deployable,
		&verdict.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding compatibility verdict: %w", err))
	}

	if err := json.Unmarshal(breaks, &verdict.Breaks); err != nil {
		panic(fmt.Errorf("error decoding compatibility verdict breaks: %w", err))
	}

	return verdict, true
}

func (this *CompatibilityRepository) insertCheck(
	ctx context.Context,
	tx pgx.Tx,
	check *model.CompatibilityCheck,
) {
	if err := tx.QueryRow(
		ctx,
		insertCompatibilityCheckQuery,
		check.ParticipantID,
		check.ContractID,
		check.Version,
		check.EnvironmentID,
		check.EnvironmentName,
		check.Deployable,
		check.Report,
	).Scan(&check.ID, &check.CreatedAt); err != nil {
		panic(fmt.Errorf("error inserting compatibility check: %w", err))
	}
}

func (this *CompatibilityRepository) insertCheckResult(
	ctx context.Context,
	tx pgx.Tx,
	result *model.CompatibilityCheckResult,
) {
	if err := tx.QueryRow(
		ctx,
		insertCompatibilityCheckResultQuery,
		result.CheckID,
		result.CounterpartName,
		result.CounterpartParticipantID,
		result.CounterpartVersion,
		result.VerdictContractIDOne,
		result.VerdictContractIDTwo,
		result.Deployable,
		result.Role,
	).Scan(&result.ID); err != nil {
		panic(fmt.Errorf("error inserting compatibility check result: %w", err))
	}
}

func (this *CompatibilityRepository) insertVerdict(
	ctx context.Context,
	tx pgx.Tx,
	verdict *model.CompatibilityVerdict,
) {
	contractIDOne, contractIDTwo := model.OrderContractPair(verdict.ContractIDOne, verdict.ContractIDTwo)

	breaks, err := json.Marshal(verdict.Breaks)
	if err != nil {
		panic(fmt.Errorf("error encoding compatibility verdict breaks: %w", err))
	}

	if _, err := tx.Exec(
		ctx,
		insertCompatibilityVerdictQuery,
		contractIDOne,
		contractIDTwo,
		breaks,
	); err != nil {
		panic(fmt.Errorf("error inserting compatibility verdict: %w", err))
	}
}
