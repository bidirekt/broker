package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

func (this *IntegrationSuite) TestSessionRepository_Create_StoresTheAdminSessionFoundByIDHash() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)

	session := &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"}
	repo.Create(ctx, session)

	found, ok := repo.FindByIDHash(ctx, []byte("admin-session"))
	this.Require().True(ok)
	this.Equal([]byte("admin-session"), found.IDHash)
	this.False(found.UserID.Valid)
	this.Equal("admin-csrf", found.CSRFToken)
	this.False(found.CreatedAt.IsZero())

	_, ok = repo.FindByIDHash(ctx, []byte("unknown-session"))
	this.False(ok)
}

func (this *IntegrationSuite) TestSessionRepository_MarkSeen_BumpsLastSeenAt() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)

	repo.Create(ctx, &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"})
	_, err := this.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() - interval '1 hour'`)
	this.Require().NoError(err)

	repo.MarkSeen(ctx, []byte("admin-session"))

	found, ok := repo.FindByIDHash(ctx, []byte("admin-session"))
	this.Require().True(ok)
	this.WithinDuration(time.Now(), found.LastSeenAt, time.Minute)
}

func (this *IntegrationSuite) TestSessionRepository_Delete_RemovesOnlyThatSession() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)

	repo.Create(ctx, &model.Session{IDHash: []byte("first-session"), CSRFToken: "first-csrf"})
	repo.Create(ctx, &model.Session{IDHash: []byte("second-session"), CSRFToken: "second-csrf"})

	repo.Delete(ctx, []byte("first-session"))

	_, ok := repo.FindByIDHash(ctx, []byte("first-session"))
	this.False(ok)
	_, ok = repo.FindByIDHash(ctx, []byte("second-session"))
	this.True(ok)
}

func (this *IntegrationSuite) TestSessionRepository_DeleteAdminSessions_KeepsUserSessions() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewSessionRepository(this.Pool)

	user := &model.User{Email: "ana@x.com", Permission: model.PermissionWrite}
	this.Require().True(users.CreateInvited(ctx, user,
		&model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	repo.Create(ctx, &model.Session{IDHash: []byte("ana-session"), UserID: null.IntFrom(user.ID), CSRFToken: "ana-csrf"})
	repo.Create(ctx, &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"})

	repo.DeleteAdminSessions(ctx)

	_, ok := repo.FindByIDHash(ctx, []byte("admin-session"))
	this.False(ok)
	found, ok := repo.FindByIDHash(ctx, []byte("ana-session"))
	this.Require().True(ok)
	this.Equal(null.IntFrom(user.ID), found.UserID)
}

func (this *IntegrationSuite) TestSessionRepository_DeleteExpired_RemovesIdleAndOldSessions() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)

	for _, idHash := range []string{"idle-session", "old-session", "fresh-session"} {
		repo.Create(ctx, &model.Session{IDHash: []byte(idHash), CSRFToken: idHash})
	}
	_, err := this.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() - interval '9 hours' WHERE id_hash = $1`, []byte("idle-session"))
	this.Require().NoError(err)
	_, err = this.Pool.Exec(ctx, `UPDATE sessions SET created_at = now() - interval '8 days' WHERE id_hash = $1`, []byte("old-session"))
	this.Require().NoError(err)

	repo.DeleteExpired(ctx, 8*time.Hour, 7*24*time.Hour)

	this.Equal(1, this.countRows("sessions"))
	_, ok := repo.FindByIDHash(ctx, []byte("fresh-session"))
	this.True(ok)
}
