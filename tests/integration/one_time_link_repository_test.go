package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	oneTimeLinkRepositoryEmail        = "ana@x.com"
	oneTimeLinkRepositoryPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"
)

func oneTimeLinkRepositoryInBackground(write func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		write()
	}()

	return panicked
}

func (this *IntegrationSuite) oneTimeLinkRepositoryAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) oneTimeLinkRepositoryCreateActiveUser() (userID int64) {
	err := this.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, permission, password_hash) VALUES ($1, 'write', $2) RETURNING id`,
		oneTimeLinkRepositoryEmail, oneTimeLinkRepositoryPasswordHash,
	).Scan(&userID)
	this.Require().NoError(err)

	return userID
}

func (this *IntegrationSuite) TestOneTimeLinkRepository_ReplaceOpen_InvalidatesThePreviousLinkAndLeavesOneOpen() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewOneTimeLinkRepository(this.Pool)

	user := &model.User{Email: oneTimeLinkRepositoryEmail, Permission: model.PermissionWrite}
	this.Require().True(users.CreateInvited(ctx, user,
		&model.OneTimeLink{TokenHash: []byte("first-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))

	email, userFound, deactivated := repo.ReplaceOpen(ctx, &model.OneTimeLink{TokenHash: []byte("second-link"), UserID: user.ID, ExpiresAt: time.Now().Add(24 * time.Hour)})
	this.True(userFound)
	this.False(deactivated)
	this.Equal(oneTimeLinkRepositoryEmail, email)

	_, _, ok := repo.FindUsable(ctx, []byte("first-link"))
	this.False(ok)

	link, found, ok := repo.FindUsable(ctx, []byte("second-link"))
	this.Require().True(ok)
	this.Equal(user.ID, link.UserID)
	this.Equal(model.LinkPurposeInvite, link.Purpose)
	this.False(link.UsedAt.Valid)
	this.Equal(oneTimeLinkRepositoryEmail, found.Email)

	var open int
	this.Require().NoError(this.Pool.QueryRow(ctx,
		`SELECT count(*) FROM one_time_links WHERE user_id = $1 AND used_at IS NULL`, user.ID,
	).Scan(&open))
	this.Equal(1, open)
}

func (this *IntegrationSuite) TestOneTimeLinkRepository_ReplaceOpen_RefusesAUserDeactivatedWhileWaitingForTheLock() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewOneTimeLinkRepository(this.Pool)
	userID := this.oneTimeLinkRepositoryCreateActiveUser()

	deactivation, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = deactivation.Rollback(ctx) }()
	_, err = deactivation.Exec(ctx, `UPDATE users SET active = false, updated_at = now() WHERE id = $1`, userID)
	this.Require().NoError(err)

	var found, deactivated bool
	replaceFinished := oneTimeLinkRepositoryInBackground(func() {
		_, found, deactivated = repo.ReplaceOpen(ctx, &model.OneTimeLink{TokenHash: []byte("ana-reset"), UserID: userID, ExpiresAt: time.Now().Add(24 * time.Hour)})
	})
	this.oneTimeLinkRepositoryAwaitLockWaiters(1)

	this.Require().NoError(deactivation.Commit(ctx))
	this.Nil(<-replaceFinished)
	this.True(found)
	this.True(deactivated)
	this.Equal(0, this.countRows("one_time_links"))

	this.Require().True(users.Reactivate(ctx, userID))
	_, _, usable := repo.FindUsable(ctx, []byte("ana-reset"))
	this.False(usable)
	this.Equal(0, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestOneTimeLinkRepository_ReplaceOpen_ReportsAnUnknownUserAndStoresNothing() {
	repo := repository.NewOneTimeLinkRepository(this.Pool)

	email, found, deactivated := repo.ReplaceOpen(context.Background(),
		&model.OneTimeLink{TokenHash: []byte("nobody-link"), UserID: 1, ExpiresAt: time.Now().Add(24 * time.Hour)},
	)

	this.Empty(email)
	this.False(found)
	this.False(deactivated)
	this.Equal(0, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestOneTimeLinkRepository_FindUsable_SkipsExpiredUsedAndDeactivatedLinks() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewOneTimeLinkRepository(this.Pool)

	expired := &model.User{Email: "expired@x.com", Permission: model.PermissionRead}
	this.Require().True(users.CreateInvited(ctx, expired,
		&model.OneTimeLink{TokenHash: []byte("expired-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(-time.Minute)},
	))

	used := &model.User{Email: "used@x.com", Permission: model.PermissionRead}
	this.Require().True(users.CreateInvited(ctx, used,
		&model.OneTimeLink{TokenHash: []byte("used-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	this.Require().True(users.SetPasswordWithLink(ctx, []byte("used-link"), "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"))

	deactivated := &model.User{Email: "deactivated@x.com", Permission: model.PermissionRead}
	this.Require().True(users.CreateInvited(ctx, deactivated,
		&model.OneTimeLink{TokenHash: []byte("deactivated-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	this.Require().True(users.Deactivate(ctx, deactivated.ID))
	_, err := this.Pool.Exec(ctx,
		`INSERT INTO one_time_links (token_hash, user_id, purpose, expires_at) VALUES ($1, $2, 'reset', now() + interval '1 day')`,
		[]byte("deactivated-reset"), deactivated.ID,
	)
	this.Require().NoError(err)

	for _, tokenHash := range []string{"expired-link", "used-link", "deactivated-reset", "unknown-link"} {
		_, _, ok := repo.FindUsable(ctx, []byte(tokenHash))
		this.False(ok, tokenHash)
	}
}

func (this *IntegrationSuite) TestOneTimeLinkRepository_DeleteUsedAndExpired_KeepsOnlyUsableLinks() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewOneTimeLinkRepository(this.Pool)

	for _, email := range []string{"expired@x.com", "used@x.com", "open@x.com"} {
		expiresAt := time.Now().Add(24 * time.Hour)
		if email == "expired@x.com" {
			expiresAt = time.Now().Add(-time.Minute)
		}
		this.Require().True(users.CreateInvited(ctx,
			&model.User{Email: email, Permission: model.PermissionRead},
			&model.OneTimeLink{TokenHash: []byte(email), Purpose: model.LinkPurposeInvite, ExpiresAt: expiresAt},
		))
	}
	this.Require().True(users.SetPasswordWithLink(ctx, []byte("used@x.com"), "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"))

	repo.DeleteUsedAndExpired(ctx)

	this.Equal(1, this.countRows("one_time_links"))
	_, _, ok := repo.FindUsable(ctx, []byte("open@x.com"))
	this.True(ok)
}
