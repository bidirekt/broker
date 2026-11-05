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
	lockLinkUserQuery = `
		SELECT
			email, active, password_hash IS NOT NULL
		FROM
			users
		WHERE
			id = $1
		FOR UPDATE
	`

	deleteOpenLinkQuery = `
		WITH deleted AS (
			DELETE FROM one_time_links
			WHERE
				user_id = $1 AND used_at IS NULL
			RETURNING expires_at
		)
		SELECT
			COALESCE(bool_or(expires_at > now()), false)
		FROM
			deleted
	`

	insertLinkQuery = `
		INSERT INTO one_time_links
			(token_hash, user_id, purpose, expires_at)
		VALUES
			($1, $2, $3, $4)
	`

	findUsableLinkQuery = `
		SELECT
			l.token_hash, l.user_id, l.purpose, l.expires_at, l.used_at,
			u.id, u.email, u.permission, u.password_hash, u.active, u.created_at, u.updated_at
		FROM
			one_time_links l
		JOIN
			users u ON u.id = l.user_id
		WHERE
			l.token_hash = $1
			AND l.used_at IS NULL
			AND l.expires_at > now()
			AND u.active
	`

	deleteUsedAndExpiredLinksQuery = `
		DELETE FROM one_time_links
		WHERE
			used_at IS NOT NULL OR expires_at <= now()
	`
)

type OneTimeLinkRepository struct {
	pool *pgxpool.Pool
}

func NewOneTimeLinkRepository(pool *pgxpool.Pool) *OneTimeLinkRepository {
	return &OneTimeLinkRepository{pool: pool}
}

// ReplaceOpen sets link.Purpose: invite for a user without a password, reset otherwise.
func (this *OneTimeLinkRepository) ReplaceOpen(ctx context.Context, link *model.OneTimeLink) (email string, found, deactivated, replacedUsable bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// Locking the user serializes two concurrent replacements, which would otherwise both delete
	// the same open link and then clash on the one-open-link-per-user index, and keeps the state
	// read here, which decides the purpose, unchanged until the new link is committed.
	var active, hasPassword bool
	err = tx.QueryRow(ctx, lockLinkUserQuery, link.UserID).Scan(&email, &active, &hasPassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, false, false
	}

	if err != nil {
		panic(fmt.Errorf("error locking link user: %w", err))
	}

	if !active {
		return email, true, true, false
	}

	link.Purpose = model.LinkPurposeReset
	if !hasPassword {
		link.Purpose = model.LinkPurposeInvite
	}

	if err := tx.QueryRow(ctx, deleteOpenLinkQuery, link.UserID).Scan(&replacedUsable); err != nil {
		panic(fmt.Errorf("error deleting open link: %w", err))
	}

	if _, err := tx.Exec(ctx, insertLinkQuery, link.TokenHash, link.UserID, link.Purpose, link.ExpiresAt); err != nil {
		panic(fmt.Errorf("error inserting link: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return email, true, false, replacedUsable
}

func (this *OneTimeLinkRepository) FindUsable(ctx context.Context, tokenHash []byte) (*model.OneTimeLink, *model.User, bool) {
	link := &model.OneTimeLink{}
	user := &model.User{}

	err := this.pool.QueryRow(ctx, findUsableLinkQuery, tokenHash).Scan(
		&link.TokenHash,
		&link.UserID,
		&link.Purpose,
		&link.ExpiresAt,
		&link.UsedAt,
		&user.ID,
		&user.Email,
		&user.Permission,
		&user.PasswordHash,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false
	}

	if err != nil {
		panic(fmt.Errorf("error finding usable link: %w", err))
	}

	return link, user, true
}

func (this *OneTimeLinkRepository) DeleteUsedAndExpired(ctx context.Context) {
	if _, err := this.pool.Exec(ctx, deleteUsedAndExpiredLinksQuery); err != nil {
		panic(fmt.Errorf("error deleting used and expired links: %w", err))
	}
}
