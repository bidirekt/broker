package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const (
	userRepositoryPasswordHash    = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"
	userRepositoryNewPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdDI$a2V5Mg"
)

func (this *IntegrationSuite) TestUserRepository_CreateInvitedWithTheSameEmailInAnotherCase_ReturnsNotCreatedAndLeavesOneUser() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)

	created := repo.CreateInvited(ctx,
		&model.User{Email: "Ana@x.com", Permission: model.PermissionWrite},
		&model.OneTimeLink{TokenHash: []byte("ana-upper-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	)
	this.Require().True(created)

	created = repo.CreateInvited(ctx,
		&model.User{Email: "ana@x.com", Permission: model.PermissionRead},
		&model.OneTimeLink{TokenHash: []byte("ana-lower-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	)
	this.False(created)

	this.Equal(1, this.countRows("users"))
	this.Equal(1, this.countRows("one_time_links"))
}

func (this *IntegrationSuite) TestUserRepository_CreateInvited_StoresAnInvitedUserFoundByEmailInAnyCase() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)

	user := &model.User{Email: "Ana@x.com", Permission: model.PermissionRead}
	link := &model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)}
	this.Require().True(repo.CreateInvited(ctx, user, link))
	this.Equal(user.ID, link.UserID)

	found, ok := repo.FindByEmail(ctx, "ANA@X.COM")
	this.Require().True(ok)
	this.Equal("Ana@x.com", found.Email)
	this.Equal(model.PermissionRead, found.Permission)
	this.Equal(model.UserInvited, found.State())

	byID, ok := repo.FindByID(ctx, user.ID)
	this.Require().True(ok)
	this.Equal(found, byID)

	_, ok = repo.FindByEmail(ctx, "bob@x.com")
	this.False(ok)
}

func (this *IntegrationSuite) TestUserRepository_List_OrdersByEmailByteWise() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)

	for _, email := range []string{"bob@x.com", "ana@x.com", "Zed@x.com"} {
		this.Require().True(repo.CreateInvited(ctx,
			&model.User{Email: email, Permission: model.PermissionRead},
			&model.OneTimeLink{TokenHash: []byte(email), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
		))
	}

	var emails []string
	for _, user := range repo.List(ctx) {
		emails = append(emails, user.Email)
	}

	this.Equal([]string{"Zed@x.com", "ana@x.com", "bob@x.com"}, emails)
}

func (this *IntegrationSuite) TestUserRepository_UpdateEmailAndPermission_ReportsTheClashAndKeepsSessions() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)
	sessions := repository.NewSessionRepository(this.Pool)

	ana := &model.User{Email: "ana@x.com", Permission: model.PermissionRead}
	this.Require().True(repo.CreateInvited(ctx, ana,
		&model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	bob := &model.User{Email: "bob@x.com", Permission: model.PermissionRead}
	this.Require().True(repo.CreateInvited(ctx, bob,
		&model.OneTimeLink{TokenHash: []byte("bob-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-session"), UserID: null.IntFrom(ana.ID), CSRFToken: "ana-csrf"})

	updated, conflict := repo.UpdateEmailAndPermission(ctx, ana.ID, "BOB@x.com", model.PermissionWrite)
	this.False(updated)
	this.True(conflict)

	updated, conflict = repo.UpdateEmailAndPermission(ctx, ana.ID, "ana.new@x.com", model.PermissionWrite)
	this.True(updated)
	this.False(conflict)

	found, ok := repo.FindByID(ctx, ana.ID)
	this.Require().True(ok)
	this.Equal("ana.new@x.com", found.Email)
	this.Equal(model.PermissionWrite, found.Permission)
	this.Equal(1, this.countRows("sessions"))

	updated, conflict = repo.UpdateEmailAndPermission(ctx, ana.ID+bob.ID, "nobody@x.com", model.PermissionRead)
	this.False(updated)
	this.False(conflict)
}

func (this *IntegrationSuite) TestUserRepository_Deactivate_WipesSessionsAndOpenLinkOnce() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)
	sessions := repository.NewSessionRepository(this.Pool)
	links := repository.NewOneTimeLinkRepository(this.Pool)

	user := &model.User{Email: "ana@x.com", Permission: model.PermissionWrite}
	this.Require().True(repo.CreateInvited(ctx, user,
		&model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-session"), UserID: null.IntFrom(user.ID), CSRFToken: "ana-csrf"})
	sessions.Create(ctx, &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"})

	this.True(repo.Deactivate(ctx, user.ID))
	this.False(repo.Deactivate(ctx, user.ID))

	found, ok := repo.FindByID(ctx, user.ID)
	this.Require().True(ok)
	this.Equal(model.UserDeactivated, found.State())
	this.Equal(0, this.countRows("one_time_links"))
	_, ok = sessions.FindByIDHash(ctx, []byte("ana-session"))
	this.False(ok)
	_, ok = sessions.FindByIDHash(ctx, []byte("admin-session"))
	this.True(ok)

	this.True(repo.Reactivate(ctx, user.ID))
	this.False(repo.Reactivate(ctx, user.ID))

	found, ok = repo.FindByID(ctx, user.ID)
	this.Require().True(ok)
	this.Equal(model.UserInvited, found.State())
	_, _, ok = links.FindUsable(ctx, []byte("ana-invite"))
	this.False(ok)
}

func (this *IntegrationSuite) TestUserRepository_SetPasswordWithLinkTwice_SetsThePasswordOnceAndWipesSessions() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)
	sessions := repository.NewSessionRepository(this.Pool)

	user := &model.User{Email: "ana@x.com", Permission: model.PermissionWrite}
	this.Require().True(repo.CreateInvited(ctx, user,
		&model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-session"), UserID: null.IntFrom(user.ID), CSRFToken: "ana-csrf"})

	this.True(repo.SetPasswordWithLink(ctx, []byte("ana-invite"), userRepositoryPasswordHash))
	this.False(repo.SetPasswordWithLink(ctx, []byte("ana-invite"), userRepositoryNewPasswordHash))

	found, ok := repo.FindByID(ctx, user.ID)
	this.Require().True(ok)
	this.Equal(model.UserActive, found.State())
	this.Equal(userRepositoryPasswordHash, found.PasswordHash.String)
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestUserRepository_SetPasswordWithLinkOfADeactivatedUserOrExpired_SetsNothing() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)
	links := repository.NewOneTimeLinkRepository(this.Pool)

	expired := &model.User{Email: "ana@x.com", Permission: model.PermissionWrite}
	this.Require().True(repo.CreateInvited(ctx, expired,
		&model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(-time.Minute)},
	))
	this.False(repo.SetPasswordWithLink(ctx, []byte("ana-invite"), userRepositoryPasswordHash))

	deactivated := &model.User{Email: "bob@x.com", Permission: model.PermissionWrite}
	this.Require().True(repo.CreateInvited(ctx, deactivated,
		&model.OneTimeLink{TokenHash: []byte("bob-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	this.Require().True(repo.Deactivate(ctx, deactivated.ID))
	links.ReplaceOpen(ctx, &model.OneTimeLink{TokenHash: []byte("bob-reset"), UserID: deactivated.ID, Purpose: model.LinkPurposeReset, ExpiresAt: time.Now().Add(24 * time.Hour)})
	this.False(repo.SetPasswordWithLink(ctx, []byte("bob-reset"), userRepositoryPasswordHash))

	for _, id := range []int64{expired.ID, deactivated.ID} {
		found, ok := repo.FindByID(ctx, id)
		this.Require().True(ok)
		this.False(found.PasswordHash.Valid)
	}
}

func (this *IntegrationSuite) TestUserRepository_ChangePasswordKeepingSession_KeepsOnlyTheGivenSession() {
	ctx := context.Background()
	repo := repository.NewUserRepository(this.Pool)
	sessions := repository.NewSessionRepository(this.Pool)

	user := &model.User{Email: "ana@x.com", Permission: model.PermissionWrite}
	this.Require().True(repo.CreateInvited(ctx, user,
		&model.OneTimeLink{TokenHash: []byte("ana-invite"), Purpose: model.LinkPurposeInvite, ExpiresAt: time.Now().Add(24 * time.Hour)},
	))
	this.Require().True(repo.SetPasswordWithLink(ctx, []byte("ana-invite"), userRepositoryPasswordHash))
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-laptop"), UserID: null.IntFrom(user.ID), CSRFToken: "laptop-csrf"})
	sessions.Create(ctx, &model.Session{IDHash: []byte("ana-phone"), UserID: null.IntFrom(user.ID), CSRFToken: "phone-csrf"})
	sessions.Create(ctx, &model.Session{IDHash: []byte("admin-session"), CSRFToken: "admin-csrf"})

	repo.ChangePasswordKeepingSession(ctx, user.ID, userRepositoryNewPasswordHash, []byte("ana-laptop"))

	found, ok := repo.FindByID(ctx, user.ID)
	this.Require().True(ok)
	this.Equal(userRepositoryNewPasswordHash, found.PasswordHash.String)
	_, ok = sessions.FindByIDHash(ctx, []byte("ana-laptop"))
	this.True(ok)
	_, ok = sessions.FindByIDHash(ctx, []byte("ana-phone"))
	this.False(ok)
	_, ok = sessions.FindByIDHash(ctx, []byte("admin-session"))
	this.True(ok)
}
