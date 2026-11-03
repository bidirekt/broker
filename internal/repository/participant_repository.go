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

	lockParticipantByNameQuery = `
		SELECT
			id
		FROM
			participants
		WHERE
			name = $1
		FOR UPDATE
	`

	participantPublishedQuery = `
		SELECT EXISTS (
			SELECT
				1
			FROM
				contracts
			WHERE
				participant_id = $1
		)
	`

	renameParticipantQuery = `
		UPDATE participants
		SET
			name = $2
		WHERE
			id = $1
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

func (this *ParticipantRepository) Rename(ctx context.Context, oldName, newName string) (found, conflict, published bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	err = tx.QueryRow(ctx, lockParticipantByNameQuery, oldName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, false
	}

	if err != nil {
		panic(fmt.Errorf("error locking participant: %w", err))
	}

	// its own statement, so its snapshot sees a first publish that committed while the lock was awaited
	if err := tx.QueryRow(ctx, participantPublishedQuery, id).Scan(&published); err != nil {
		panic(fmt.Errorf("error checking whether the participant is published: %w", err))
	}

	if published {
		return true, false, true
	}

	_, err = tx.Exec(ctx, renameParticipantQuery, id, newName)
	if isUniqueViolation(err) {
		return false, true, false
	}

	if err != nil {
		panic(fmt.Errorf("error renaming participant: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true, false, false
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
