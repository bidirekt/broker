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
	existsEnvironmentByNameQuery = `
		SELECT EXISTS(
			SELECT
				1
			FROM
				environments
			WHERE
				name = $1
		)
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
)

type EnvironmentRepository struct {
	pool *pgxpool.Pool
}

func NewEnvironmentRepository(pool *pgxpool.Pool) *EnvironmentRepository {
	return &EnvironmentRepository{pool: pool}
}

func (this *EnvironmentRepository) ExistsByName(ctx context.Context, name string) bool {
	var exists bool
	if err := this.pool.QueryRow(ctx, existsEnvironmentByNameQuery, name).Scan(&exists); err != nil {
		panic(fmt.Errorf("error finding environment by name: %w", err))
	}
	return exists
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

func (this *EnvironmentRepository) Create(ctx context.Context, e *model.Environment) {
	if err := this.pool.QueryRow(ctx, insertEnvironmentQuery, e.Name).Scan(&e.ID); err != nil {
		panic(fmt.Errorf("error inserting environment: %w", err))
	}
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
