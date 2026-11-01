package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

func (this *IntegrationSuite) TestStartup_DeletesAdminSessionsAndExpiredSessionsAndLinks() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	sessions := repository.NewSessionRepository(this.Pool)
	links := repository.NewOneTimeLinkRepository(this.Pool)

	ana := &model.User{Email: "ana@x.com", Permission: model.PermissionWrite}
	this.Require().True(users.CreateInvited(ctx, ana,
		&model.OneTimeLink{TokenHash: []byte("ana-open-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	bia := &model.User{Email: "bia@x.com", Permission: model.PermissionRead}
	this.Require().True(users.CreateInvited(ctx, bia,
		&model.OneTimeLink{TokenHash: []byte("bia-expired-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(-time.Minute)},
	))
	cid := &model.User{Email: "cid@x.com", Permission: model.PermissionRead}
	this.Require().True(users.CreateInvited(ctx, cid,
		&model.OneTimeLink{TokenHash: []byte("cid-used-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	_, err := this.Pool.Exec(ctx, `UPDATE one_time_links SET used_at = now() WHERE token_hash = $1`, []byte("cid-used-link"))
	this.Require().NoError(err)

	this.Require().True(sessions.Create(ctx, &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"}, ""))
	for _, idHash := range []string{"ana-fresh-session", "ana-idle-session", "ana-old-session"} {
		_, err = this.Pool.Exec(ctx, `INSERT INTO sessions (id_hash, user_id, csrf_token) VALUES ($1, $2, $3)`, []byte(idHash), ana.ID, idHash+"-csrf")
		this.Require().NoError(err)
	}
	_, err = this.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() - interval '9 hours' WHERE id_hash = $1`, []byte("ana-idle-session"))
	this.Require().NoError(err)
	_, err = this.Pool.Exec(ctx, `UPDATE sessions SET created_at = now() - interval '8 days' WHERE id_hash = $1`, []byte("ana-old-session"))
	this.Require().NoError(err)

	restarted, err := internal.Run()
	this.Require().NoError(err)
	restarted.Pool.Close()

	this.Equal(1, this.countRows("sessions"))
	_, found := sessions.FindByIDHash(ctx, []byte("ana-fresh-session"))
	this.True(found)

	this.Equal(1, this.countRows("one_time_links"))
	_, _, found = links.FindUsable(ctx, []byte("ana-open-link"))
	this.True(found)
}

func (this *IntegrationSuite) TestStartup_GivesUpOnACleanupThatCannotGetItsLockAndSaysWhy() {
	ctx := context.Background()
	sessionsLock, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = sessionsLock.Rollback(ctx) }()
	_, err = sessionsLock.Exec(ctx, `LOCK TABLE sessions IN ACCESS EXCLUSIVE MODE`)
	this.Require().NoError(err)

	finished := make(chan error, 1)
	go func() {
		restarted, err := internal.Run()
		if restarted != nil {
			restarted.Pool.Close()
		}
		finished <- err
	}()

	select {
	case err := <-finished:
		this.ErrorContains(err, "failed to delete stale sessions and links")
	case <-time.After(time.Minute):
		this.Fail("the start-up cleanup still waits for the sessions lock")
	}
}
