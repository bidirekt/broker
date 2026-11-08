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
	lockAPIKeyOwnerQuery = `
		SELECT
			id, email, permission, password_hash, active, created_at, updated_at
		FROM
			users
		WHERE
			id = $1
		FOR SHARE
	`

	insertAPIKeyQuery = `
		INSERT INTO api_keys
			(user_id, key_hash, display_prefix, name, permission, expires_at)
		VALUES
			($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	countUserAPIKeysQuery = `
		SELECT
			count(*)
		FROM
			api_keys
		WHERE
			user_id = $1
	`

	listUserAPIKeysQuery = `
		SELECT
			id, user_id, key_hash, display_prefix, name, permission, created_at, last_used_at, expires_at
		FROM
			api_keys
		WHERE
			user_id = $1
		ORDER BY
			created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`

	findAPIKeyByHashQuery = `
		SELECT
			k.id, k.user_id, k.key_hash, k.display_prefix, k.name, k.permission, k.created_at, k.last_used_at, k.expires_at,
			u.id, u.email, u.permission, u.password_hash, u.active, u.created_at, u.updated_at
		FROM
			api_keys k
		JOIN
			users u ON u.id = k.user_id
		WHERE
			k.key_hash = $1
	`

	deleteAPIKeyByIDAndUserQuery = `
		DELETE FROM api_keys
		WHERE
			id = $1 AND user_id = $2
		RETURNING name
	`

	markAPIKeyUsedQuery = `
		UPDATE api_keys
		SET
			last_used_at = now()
		WHERE
			id = $1
	`

	deleteExpiredAPIKeysQuery = `
		DELETE FROM api_keys
		WHERE
			expires_at <= now()
	`
)

type APIKeyRepository struct {
	pool *pgxpool.Pool
}

func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

func (this *APIKeyRepository) Create(ctx context.Context, key *model.APIKey) (created bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, lockAPIKeyOwnerQuery, key.UserID)
	if err != nil {
		panic(fmt.Errorf("error locking API key owner: %w", err))
	}

	owner, err := pgx.CollectExactlyOneRow(rows, scanUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error locking API key owner: %w", err))
	}

	if owner.State() != model.UserActive || !owner.Permission.Includes(key.Permission) {
		return false
	}

	err = tx.QueryRow(ctx, insertAPIKeyQuery, key.UserID, key.KeyHash, key.DisplayPrefix, key.Name, key.Permission, key.ExpiresAt).
		Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		panic(fmt.Errorf("error inserting API key: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true
}

func (this *APIKeyRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) (keys []*model.APIKey, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countUserAPIKeysQuery, userID).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting user API keys: %w", err))
	}

	rows, err := tx.Query(ctx, listUserAPIKeysQuery, userID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing user API keys: %w", err))
	}

	keys, err = pgx.AppendRows(make([]*model.APIKey, 0), rows, func(row pgx.CollectableRow) (*model.APIKey, error) {
		key := &model.APIKey{}
		err := row.Scan(
			&key.ID,
			&key.UserID,
			&key.KeyHash,
			&key.DisplayPrefix,
			&key.Name,
			&key.Permission,
			&key.CreatedAt,
			&key.LastUsedAt,
			&key.ExpiresAt,
		)
		return key, err
	})
	if err != nil {
		panic(fmt.Errorf("error listing user API keys: %w", err))
	}

	return keys, total
}

func (this *APIKeyRepository) FindByHash(ctx context.Context, keyHash []byte) (*model.APIKey, *model.User, bool) {
	key := &model.APIKey{}
	owner := &model.User{}

	err := this.pool.QueryRow(ctx, findAPIKeyByHashQuery, keyHash).Scan(
		&key.ID,
		&key.UserID,
		&key.KeyHash,
		&key.DisplayPrefix,
		&key.Name,
		&key.Permission,
		&key.CreatedAt,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&owner.ID,
		&owner.Email,
		&owner.Permission,
		&owner.PasswordHash,
		&owner.Active,
		&owner.CreatedAt,
		&owner.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false
	}

	if err != nil {
		panic(fmt.Errorf("error finding API key: %w", err))
	}

	return key, owner, true
}

func (this *APIKeyRepository) DeleteByIDAndUser(ctx context.Context, id, userID int64) (name string, deleted bool) {
	err := this.pool.QueryRow(ctx, deleteAPIKeyByIDAndUserQuery, id, userID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false
	}

	if err != nil {
		panic(fmt.Errorf("error deleting API key: %w", err))
	}

	return name, true
}

func (this *APIKeyRepository) MarkUsed(ctx context.Context, id int64) {
	if _, err := this.pool.Exec(ctx, markAPIKeyUsedQuery, id); err != nil {
		panic(fmt.Errorf("error marking API key used: %w", err))
	}
}

func (this *APIKeyRepository) DeleteExpired(ctx context.Context) {
	if _, err := this.pool.Exec(ctx, deleteExpiredAPIKeysQuery); err != nil {
		panic(fmt.Errorf("error deleting expired API keys: %w", err))
	}
}
