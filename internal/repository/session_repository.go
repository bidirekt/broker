package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertSessionQuery = `
		INSERT INTO sessions
			(id_hash, user_id, csrf_token)
		VALUES
			($1, $2, $3)
		RETURNING created_at, last_seen_at
	`

	findSessionByIDHashQuery = `
		SELECT
			id_hash, user_id, csrf_token, created_at, last_seen_at
		FROM
			sessions
		WHERE
			id_hash = $1
	`

	markSessionSeenQuery = `
		UPDATE sessions
		SET
			last_seen_at = now()
		WHERE
			id_hash = $1
	`

	deleteSessionQuery = `
		DELETE FROM sessions
		WHERE
			id_hash = $1
	`

	deleteAdminSessionsQuery = `
		DELETE FROM sessions
		WHERE
			user_id IS NULL
	`

	deleteExpiredSessionsQuery = `
		DELETE FROM sessions
		WHERE
			last_seen_at <= now() - make_interval(secs => $1)
			OR created_at <= now() - make_interval(secs => $2)
	`
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (this *SessionRepository) Create(ctx context.Context, session *model.Session) {
	err := this.pool.QueryRow(ctx, insertSessionQuery, session.IDHash, session.UserID, session.CSRFToken).
		Scan(&session.CreatedAt, &session.LastSeenAt)
	if err != nil {
		panic(fmt.Errorf("error inserting session: %w", err))
	}
}

func (this *SessionRepository) FindByIDHash(ctx context.Context, idHash []byte) (*model.Session, bool) {
	session := &model.Session{}
	err := this.pool.QueryRow(ctx, findSessionByIDHashQuery, idHash).Scan(
		&session.IDHash,
		&session.UserID,
		&session.CSRFToken,
		&session.CreatedAt,
		&session.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}

	if err != nil {
		panic(fmt.Errorf("error finding session: %w", err))
	}

	return session, true
}

func (this *SessionRepository) MarkSeen(ctx context.Context, idHash []byte) {
	if _, err := this.pool.Exec(ctx, markSessionSeenQuery, idHash); err != nil {
		panic(fmt.Errorf("error marking session seen: %w", err))
	}
}

func (this *SessionRepository) Delete(ctx context.Context, idHash []byte) {
	if _, err := this.pool.Exec(ctx, deleteSessionQuery, idHash); err != nil {
		panic(fmt.Errorf("error deleting session: %w", err))
	}
}

func (this *SessionRepository) DeleteAdminSessions(ctx context.Context) {
	if _, err := this.pool.Exec(ctx, deleteAdminSessionsQuery); err != nil {
		panic(fmt.Errorf("error deleting admin sessions: %w", err))
	}
}

func (this *SessionRepository) DeleteExpired(ctx context.Context, idleLimit, absoluteLimit time.Duration) {
	if _, err := this.pool.Exec(ctx, deleteExpiredSessionsQuery, idleLimit.Seconds(), absoluteLimit.Seconds()); err != nil {
		panic(fmt.Errorf("error deleting expired sessions: %w", err))
	}
}
