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
	findParticipantByNameQuery = `
		SELECT
			id, name
		FROM
			participants
		WHERE
			name = $1
	`

	insertParticipantQuery = `
		INSERT INTO participants
			(name)
		VALUES
			($1)
		ON CONFLICT (name) DO NOTHING
		RETURNING id
	`

	hasContractForVersionQuery = `
		SELECT EXISTS(SELECT 1 FROM contract_versions WHERE participant_id = $1 AND version = $2)
	`

	listParticipantNamesQuery = `
		SELECT
			name
		FROM
			participants
		ORDER BY
			name COLLATE "C"
	`
)

type ParticipantRepository struct {
	pool *pgxpool.Pool
}

func NewParticipantRepository(pool *pgxpool.Pool) *ParticipantRepository {
	return &ParticipantRepository{pool: pool}
}

func (this *ParticipantRepository) FindByName(ctx context.Context, name string) (*model.Participant, bool) {
	p := &model.Participant{}
	err := this.pool.QueryRow(ctx, findParticipantByNameQuery, name).Scan(&p.ID, &p.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding participant by name: %w", err))
	}
	return p, true
}

func (this *ParticipantRepository) Create(ctx context.Context, p *model.Participant) (created bool) {
	err := this.pool.QueryRow(ctx, insertParticipantQuery, p.Name).Scan(&p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error inserting participant: %w", err))
	}

	return true
}

func (this *ParticipantRepository) ListNames(ctx context.Context) []string {
	rows, err := this.pool.Query(ctx, listParticipantNamesQuery)
	if err != nil {
		panic(fmt.Errorf("error listing participant names: %w", err))
	}

	names, err := pgx.AppendRows(make([]string, 0), rows, pgx.RowTo[string])
	if err != nil {
		panic(fmt.Errorf("error listing participant names: %w", err))
	}

	return names
}

func (this *ParticipantRepository) HasContractForVersion(ctx context.Context, participantID int64, version string) bool {
	var exists bool

	if err := this.pool.QueryRow(ctx, hasContractForVersionQuery, participantID, version).Scan(&exists); err != nil {
		panic(fmt.Errorf("error checking contract version: %w", err))
	}

	return exists
}
