package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertSessionQuery = `
		INSERT INTO sessions
			(id_hash, user_id, csrf_token)
		SELECT
			$1, id, $3
		FROM
			users
		WHERE
			id = $2 AND active AND password_hash = $4
		FOR SHARE
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

	// a CTE read only in RETURNING would return NULL; RETURNING OLD needs Postgres 18 and the compose runs 17
	markSessionSeenTakingMessageQuery = `
		UPDATE sessions s
		SET
			last_seen_at = now(), message = NULL
		FROM
			(SELECT id_hash, message FROM sessions WHERE id_hash = $1 FOR UPDATE) prior
		WHERE
			s.id_hash = prior.id_hash
		RETURNING prior.message
	`

	setSessionMessageQuery = `
		UPDATE sessions
		SET
			message = $2
		WHERE
			id_hash = $1
	`

	deleteSessionQuery = `
		DELETE FROM sessions
		WHERE
			id_hash = $1
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

func (this *SessionRepository) Create(ctx context.Context, session *model.Session, verifiedPasswordHash string) (created bool) {
	// the user is read again under a share lock, so a deactivation or password change since the
	// sign-in read it either makes this insert nothing or waits for it and then deletes the session
	err := this.pool.QueryRow(ctx, insertSessionQuery, session.IDHash, session.UserID, session.CSRFToken, verifiedPasswordHash).Scan(
		&session.CreatedAt,
		&session.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error inserting session: %w", err))
	}

	return true
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

func (this *SessionRepository) MarkSeenTakingMessage(ctx context.Context, idHash []byte) (message string) {
	var taken null.String
	err := this.pool.QueryRow(ctx, markSessionSeenTakingMessageQuery, idHash).Scan(&taken)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}

	if err != nil {
		panic(fmt.Errorf("error marking session seen taking its message: %w", err))
	}

	return taken.String
}

func (this *SessionRepository) SetMessage(ctx context.Context, idHash []byte, message string) {
	if _, err := this.pool.Exec(ctx, setSessionMessageQuery, idHash, message); err != nil {
		panic(fmt.Errorf("error setting session message: %w", err))
	}
}

func (this *SessionRepository) Delete(ctx context.Context, idHash []byte) {
	if _, err := this.pool.Exec(ctx, deleteSessionQuery, idHash); err != nil {
		panic(fmt.Errorf("error deleting session: %w", err))
	}
}

func (this *SessionRepository) DeleteExpired(ctx context.Context, idleLimit, absoluteLimit time.Duration) {
	if _, err := this.pool.Exec(ctx, deleteExpiredSessionsQuery, idleLimit.Seconds(), absoluteLimit.Seconds()); err != nil {
		panic(fmt.Errorf("error deleting expired sessions: %w", err))
	}
}
