package components

import (
	"context"
	"fmt"
	"time"

	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

const startupCleanupTimeout = 5 * time.Second

func deleteStaleSessionsAndLinks(pool *pgxpool.Pool) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), startupCleanupTimeout)
	defer cancel()

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("failed to delete stale sessions and links: %v", recovered)
		}
	}()

	sessionRepository := repository.NewSessionRepository(pool)

	sessionRepository.DeleteAdminSessions(ctx)
	sessionRepository.DeleteExpired(ctx, sessions.IdleLimit, sessions.AbsoluteLimit)
	repository.NewOneTimeLinkRepository(pool).DeleteUsedAndExpired(ctx)

	return nil
}
