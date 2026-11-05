package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	sessionRepositoryPasswordHash    = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"
	sessionRepositoryNewPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdDI$a2V5Mg"
)

func sessionRepositoryInBackground(write func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		write()
	}()

	return panicked
}

func (this *IntegrationSuite) sessionRepositoryAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) sessionRepositoryCreateActiveUser() (userID int64) {
	err := this.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, permission, password_hash) VALUES ('ana@x.com', 'write', $1) RETURNING id`,
		sessionRepositoryPasswordHash,
	).Scan(&userID)
	this.Require().NoError(err)

	return userID
}

func sessionRepositoryUserSession(idHash string, userID int64) *model.Session {
	return &model.Session{IDHash: []byte(idHash), UserID: userID, CSRFToken: idHash + "-csrf"}
}

func (this *IntegrationSuite) TestSessionRepository_Create_StoresTheSessionOfAnActiveUserWithTheVerifiedPassword() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	session := sessionRepositoryUserSession("ana-session", userID)
	this.True(repo.Create(ctx, session, sessionRepositoryPasswordHash))
	this.False(session.CreatedAt.IsZero())

	found, ok := repo.FindByIDHash(ctx, []byte("ana-session"))
	this.Require().True(ok)
	this.Equal(userID, found.UserID)
	this.Equal("ana-session-csrf", found.CSRFToken)
}

func (this *IntegrationSuite) TestSessionRepository_CreateAfterTheUserWasDeactivated_StoresNothingEvenAfterReactivation() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	signInRead, found := users.FindByID(ctx, userID)
	this.Require().True(found)
	this.Require().True(users.Deactivate(ctx, userID))

	this.False(repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), signInRead.PasswordHash.String))
	this.Require().True(users.Reactivate(ctx, userID))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionRepository_CreateAfterTheProfilePasswordChanged_StoresNothing() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()
	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("ana-laptop", userID), sessionRepositoryPasswordHash))

	signInRead, found := users.FindByID(ctx, userID)
	this.Require().True(found)
	this.Require().True(users.ChangePasswordKeepingSession(ctx, userID, signInRead.PasswordHash.String, sessionRepositoryNewPasswordHash, []byte("ana-laptop")))

	this.False(repo.Create(ctx, sessionRepositoryUserSession("ana-phone", userID), signInRead.PasswordHash.String))
	_, ok := repo.FindByIDHash(ctx, []byte("ana-phone"))
	this.False(ok)
	this.Equal(1, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionRepository_CreateAfterAResetLinkSetAnotherPassword_StoresNothing() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()
	_, found, deactivated, _ := repository.NewOneTimeLinkRepository(this.Pool).ReplaceOpen(ctx,
		&model.OneTimeLink{TokenHash: []byte("ana-reset"), UserID: userID, ExpiresAt: time.Now().Add(24 * time.Hour)},
	)
	this.Require().True(found)
	this.Require().False(deactivated)

	signInRead, found := users.FindByID(ctx, userID)
	this.Require().True(found)
	this.Require().True(users.SetPasswordWithLink(ctx, []byte("ana-reset"), sessionRepositoryNewPasswordHash))

	this.False(repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), signInRead.PasswordHash.String))
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionRepository_CreateWhileADeactivationIsOpen_WaitsForItAndStoresNothing() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	deactivation, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = deactivation.Rollback(ctx) }()
	_, err = deactivation.Exec(ctx, `UPDATE users SET active = false, updated_at = now() WHERE id = $1`, userID)
	this.Require().NoError(err)

	var created bool
	createFinished := sessionRepositoryInBackground(func() {
		created = repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), sessionRepositoryPasswordHash)
	})
	this.sessionRepositoryAwaitLockWaiters(1)

	this.Require().NoError(deactivation.Commit(ctx))
	this.Nil(<-createFinished)
	this.False(created)
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionRepository_CreateCommittedFirst_IsDeletedByTheDeactivationThatWaitedForIt() {
	ctx := context.Background()
	users := repository.NewUserRepository(this.Pool)
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	sameIDHash, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = sameIDHash.Rollback(ctx) }()
	_, err = sameIDHash.Exec(ctx, `INSERT INTO sessions (id_hash, user_id, csrf_token) VALUES ($1, $2, 'held-csrf')`, []byte("ana-session"), userID)
	this.Require().NoError(err)

	var created bool
	createFinished := sessionRepositoryInBackground(func() {
		created = repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), sessionRepositoryPasswordHash)
	})
	this.sessionRepositoryAwaitLockWaiters(1)

	var deactivated bool
	deactivateFinished := sessionRepositoryInBackground(func() {
		deactivated = users.Deactivate(ctx, userID)
	})
	this.sessionRepositoryAwaitLockWaiters(2)

	this.Require().NoError(sameIDHash.Rollback(ctx))
	this.Nil(<-createFinished)
	this.Nil(<-deactivateFinished)
	this.True(created)
	this.True(deactivated)
	this.Equal(0, this.countRows("sessions"))
}

func (this *IntegrationSuite) TestSessionRepository_MarkSeen_BumpsLastSeenAt() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), sessionRepositoryPasswordHash))
	_, err := this.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() - interval '1 hour'`)
	this.Require().NoError(err)

	repo.MarkSeen(ctx, []byte("ana-session"))

	found, ok := repo.FindByIDHash(ctx, []byte("ana-session"))
	this.Require().True(ok)
	this.WithinDuration(time.Now(), found.LastSeenAt, time.Minute)
}

func (this *IntegrationSuite) TestSessionRepository_Delete_RemovesOnlyThatSession() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("first-session", userID), sessionRepositoryPasswordHash))
	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("second-session", userID), sessionRepositoryPasswordHash))

	repo.Delete(ctx, []byte("first-session"))

	_, ok := repo.FindByIDHash(ctx, []byte("first-session"))
	this.False(ok)
	_, ok = repo.FindByIDHash(ctx, []byte("second-session"))
	this.True(ok)
}

func (this *IntegrationSuite) TestSessionRepository_DeleteExpired_RemovesIdleAndOldSessions() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	for _, idHash := range []string{"idle-session", "old-session", "fresh-session"} {
		this.Require().True(repo.Create(ctx, sessionRepositoryUserSession(idHash, userID), sessionRepositoryPasswordHash))
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

func (this *IntegrationSuite) TestSessionRepository_MarkSeenTakingMessage_ReturnsTheMessageOnceAndBumpsLastSeenAt() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), sessionRepositoryPasswordHash))
	repo.SetMessage(ctx, []byte("ana-session"), "participant created")
	_, err := this.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() - interval '1 hour'`)
	this.Require().NoError(err)

	this.Equal("participant created", repo.MarkSeenTakingMessage(ctx, []byte("ana-session")))

	found, ok := repo.FindByIDHash(ctx, []byte("ana-session"))
	this.Require().True(ok)
	this.WithinDuration(time.Now(), found.LastSeenAt, time.Minute)

	this.Empty(repo.MarkSeenTakingMessage(ctx, []byte("ana-session")))
}

func (this *IntegrationSuite) TestSessionRepository_MarkSeenTakingMessage_TakesOnlyThatSessionsMessage() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("ana-laptop", userID), sessionRepositoryPasswordHash))
	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("ana-phone", userID), sessionRepositoryPasswordHash))
	repo.SetMessage(ctx, []byte("ana-laptop"), "participant created")
	repo.SetMessage(ctx, []byte("ana-phone"), "environment created")

	this.Equal("participant created", repo.MarkSeenTakingMessage(ctx, []byte("ana-laptop")))
	this.Equal("environment created", repo.MarkSeenTakingMessage(ctx, []byte("ana-phone")))
}

func (this *IntegrationSuite) TestSessionRepository_MarkSeenTakingMessageOfAnUnknownSession_ReturnsNone() {
	repo := repository.NewSessionRepository(this.Pool)

	this.Empty(repo.MarkSeenTakingMessage(context.Background(), []byte("no-such-session")))
}

func (this *IntegrationSuite) TestSessionRepository_MarkSeen_KeepsTheMessage() {
	ctx := context.Background()
	repo := repository.NewSessionRepository(this.Pool)
	userID := this.sessionRepositoryCreateActiveUser()

	this.Require().True(repo.Create(ctx, sessionRepositoryUserSession("ana-session", userID), sessionRepositoryPasswordHash))
	repo.SetMessage(ctx, []byte("ana-session"), "participant created")

	repo.MarkSeen(ctx, []byte("ana-session"))

	this.Equal("participant created", repo.MarkSeenTakingMessage(ctx, []byte("ana-session")))
}
