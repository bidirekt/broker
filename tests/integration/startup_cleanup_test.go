package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
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

	sessions.Create(ctx, &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"})
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-fresh-session"), UserID: null.IntFrom(ana.ID), CSRFToken: "fresh-csrf"})
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-idle-session"), UserID: null.IntFrom(ana.ID), CSRFToken: "idle-csrf"})
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-old-session"), UserID: null.IntFrom(ana.ID), CSRFToken: "old-csrf"})
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
