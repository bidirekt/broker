package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	existsParticipantByNameQuery = `
		SELECT EXISTS(
			SELECT
				1
			FROM 
				participants
			WHERE 
				name = $1
		)
	`

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
		RETURNING id
	`

	renameParticipantQuery = `
		UPDATE participants
		SET
			name = $2
		WHERE
			name = $1
		RETURNING id
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

func (r *ParticipantRepository) ExistsByName(ctx context.Context, name string) bool {
	var exists bool
	if err := r.pool.QueryRow(ctx, existsParticipantByNameQuery, name).Scan(&exists); err != nil {
		panic(fmt.Errorf("error finding participant by name: %w", err))
	}
	return exists
}

func (r *ParticipantRepository) FindByName(ctx context.Context, name string) (*model.Participant, bool) {
	p := &model.Participant{}
	err := r.pool.QueryRow(ctx, findParticipantByNameQuery, name).Scan(&p.ID, &p.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding participant by name: %w", err))
	}
	return p, true
}

func (r *ParticipantRepository) Create(ctx context.Context, p *model.Participant) {
	if err := r.pool.QueryRow(ctx, insertParticipantQuery, p.Name).Scan(&p.ID); err != nil {
		panic(fmt.Errorf("error inserting participant: %w", err))
	}
}

func (r *ParticipantRepository) Rename(ctx context.Context, oldName, newName string) (found, conflict bool) {
	var id int64
	err := r.pool.QueryRow(ctx, renameParticipantQuery, oldName, newName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return false, true
	}

	if err != nil {
		panic(fmt.Errorf("error renaming participant: %w", err))
	}

	return true, false
}

func (this *ParticipantRepository) ListNames(ctx context.Context) []string {
	rows, err := this.pool.Query(ctx, listParticipantNamesQuery)
	if err != nil {
		panic(fmt.Errorf("error listing participant names: %w", err))
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			panic(fmt.Errorf("error scanning participant name: %w", err))
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error listing participant names: %w", err))
	}

	return names
}
