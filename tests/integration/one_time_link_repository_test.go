package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const oneTimeLinkRepositoryEmail = "ana@x.com"

func (this *IntegrationSuite) TestOneTimeLinkRepository_ReplaceOpen_InvalidatesThePreviousLinkAndLeavesOneOpen() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewOneTimeLinkRepository(this.Pool)

	user := &model.User{Email: oneTimeLinkRepositoryEmail, Permission: model.PermissionWrite}
	this.Require().True(users.CreateInvited(ctx, user,
		&model.OneTimeLink{TokenHash: []byte("first-link"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))

	repo.ReplaceOpen(ctx, &model.OneTimeLink{TokenHash: []byte("second-link"), UserID: user.ID, Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)})

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
	repo.ReplaceOpen(ctx, &model.OneTimeLink{TokenHash: []byte("deactivated-reset"), UserID: deactivated.ID, Purpose: model.LinkPurposeReset, ExpiresAt: time.Now().Add(24 * time.Hour)})

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
