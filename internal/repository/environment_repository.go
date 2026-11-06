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
	findEnvironmentByIDQuery = `
		SELECT id, name FROM environments WHERE id = $1
	`

	findEnvironmentByNameQuery = `
		SELECT
			id, name
		FROM
			environments
		WHERE
			name = $1
	`

	insertEnvironmentQuery = `
		INSERT INTO environments
			(name)
		VALUES
			($1)
		ON CONFLICT (name) DO NOTHING
		RETURNING id
	`

	listEnvironmentNamesQuery = `
		SELECT
			name
		FROM
			environments
		ORDER BY
			name COLLATE "C"
	`

	renameEnvironmentByNameQuery = `
		UPDATE
			environments
		SET
			name = $2
		WHERE
			name = $1
		RETURNING id
	`

	renameEnvironmentByIDQuery = `
		UPDATE
			environments
		SET
			name = $2
		WHERE
			id = $1
		RETURNING id
	`
)

type EnvironmentRepository struct {
	pool *pgxpool.Pool
}

func NewEnvironmentRepository(pool *pgxpool.Pool) *EnvironmentRepository {
	return &EnvironmentRepository{pool: pool}
}

func (this *EnvironmentRepository) FindByName(ctx context.Context, name string) (*model.Environment, bool) {
	e := &model.Environment{}
	err := this.pool.QueryRow(ctx, findEnvironmentByNameQuery, name).Scan(&e.ID, &e.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding environment by name: %w", err))
	}
	return e, true
}

func (this *EnvironmentRepository) FindByID(ctx context.Context, id int64) (*model.Environment, bool) {
	e := &model.Environment{}
	err := this.pool.QueryRow(ctx, findEnvironmentByIDQuery, id).Scan(&e.ID, &e.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding environment by id: %w", err))
	}
	return e, true
}

func (this *EnvironmentRepository) Create(ctx context.Context, e *model.Environment) (created bool) {
	err := this.pool.QueryRow(ctx, insertEnvironmentQuery, e.Name).Scan(&e.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error inserting environment: %w", err))
	}

	return true
}

func (this *EnvironmentRepository) ListNames(ctx context.Context) []string {
	rows, err := this.pool.Query(ctx, listEnvironmentNamesQuery)
	if err != nil {
		panic(fmt.Errorf("error listing environment names: %w", err))
	}

	names, err := pgx.AppendRows(make([]string, 0), rows, pgx.RowTo[string])
	if err != nil {
		panic(fmt.Errorf("error listing environment names: %w", err))
	}

	return names
}

func (this *EnvironmentRepository) RenameByName(ctx context.Context, oldName, newName string) (found, conflict bool) {
	return this.rename(ctx, renameEnvironmentByNameQuery, oldName, newName)
}

func (this *EnvironmentRepository) RenameByID(ctx context.Context, id int64, newName string) (found, conflict bool) {
	return this.rename(ctx, renameEnvironmentByIDQuery, id, newName)
}

func (this *EnvironmentRepository) rename(ctx context.Context, query string, key any, newName string) (found, conflict bool) {
	var id int64
	err := this.pool.QueryRow(ctx, query, key, newName).Scan(&id)
	if isUniqueViolation(err) {
		return true, true
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return false, false
	}

	if err != nil {
		panic(fmt.Errorf("error renaming environment: %w", err))
	}

	return true, false
}
