package integration_test

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const apiKeyRepositoryPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"

func apiKeyRepositoryInBackground(write func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		write()
	}()

	return panicked
}

func (this *IntegrationSuite) apiKeyRepositoryAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) apiKeyRepositoryCreateUser(email string, permission model.Permission) (userID int64) {
	err := this.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, permission, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, permission, apiKeyRepositoryPasswordHash,
	).Scan(&userID)
	this.Require().NoError(err)

	return userID
}

func apiKeyRepositoryKey(name string, userID int64, permission model.Permission) *model.APIKey {
	return &model.APIKey{
		UserID:        userID,
		KeyHash:       []byte(name + "-hash"),
		DisplayPrefix: "bdk_" + name,
		Name:          name,
		Permission:    permission,
	}
}

func apiKeyRepositoryNamesOf(keys []*model.APIKey) (names []string) {
	for _, key := range keys {
		names = append(names, key.Name)
	}

	return names
}

func (this *IntegrationSuite) apiKeyRepositoryLastUsedAt(keyID int64) (lastUsedAt null.Time) {
	err := this.Pool.QueryRow(context.Background(), `SELECT last_used_at FROM api_keys WHERE id = $1`, keyID).Scan(&lastUsedAt)
	this.Require().NoError(err)

	return lastUsedAt
}

func (this *IntegrationSuite) TestAPIKeyRepository_Create_StoresTheKeyFoundByItsHashWithItsOwner() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	expiresAt := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)

	key := apiKeyRepositoryKey("ana-laptop", userID, model.PermissionWrite)
	key.ExpiresAt = null.TimeFrom(expiresAt)
	this.Require().True(repo.Create(ctx, key))
	this.NotZero(key.ID)
	this.WithinDuration(time.Now(), key.CreatedAt, time.Minute)

	found, owner, ok := repo.FindByHash(ctx, []byte("ana-laptop-hash"))
	this.Require().True(ok)
	this.Equal(key.ID, found.ID)
	this.Equal(userID, found.UserID)
	this.Equal([]byte("ana-laptop-hash"), found.KeyHash)
	this.Equal("bdk_ana-laptop", found.DisplayPrefix)
	this.Equal("ana-laptop", found.Name)
	this.Equal(model.PermissionWrite, found.Permission)
	this.True(key.CreatedAt.Equal(found.CreatedAt))
	this.False(found.LastUsedAt.Valid)
	this.True(found.ExpiresAt.Valid)
	this.True(expiresAt.Equal(found.ExpiresAt.Time))
	this.Equal(userID, owner.ID)
	this.Equal("ana@x.com", owner.Email)
	this.Equal(model.UserActive, owner.State())

	_, _, ok = repo.FindByHash(ctx, []byte("unknown-hash"))
	this.False(ok)
}

func (this *IntegrationSuite) TestAPIKeyRepository_CreateWithoutExpiry_StoresAKeyThatNeverExpires() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionRead)

	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("ana-ci", userID, model.PermissionRead)))

	found, _, ok := repo.FindByHash(ctx, []byte("ana-ci-hash"))
	this.Require().True(ok)
	this.False(found.ExpiresAt.Valid)
}

func (this *IntegrationSuite) TestAPIKeyRepository_Create_StoresOnlyAKeyTheOwnersPermissionCovers() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	owners := map[model.Permission]int64{
		model.PermissionRead:  this.apiKeyRepositoryCreateUser("reader@x.com", model.PermissionRead),
		model.PermissionWrite: this.apiKeyRepositoryCreateUser("writer@x.com", model.PermissionWrite),
		model.PermissionAdmin: this.adminID,
	}

	for _, attempt := range []struct {
		owner   model.Permission
		key     model.Permission
		created bool
	}{
		{owner: model.PermissionRead, key: model.PermissionRead, created: true},
		{owner: model.PermissionRead, key: model.PermissionWrite, created: false},
		{owner: model.PermissionWrite, key: model.PermissionRead, created: true},
		{owner: model.PermissionWrite, key: model.PermissionWrite, created: true},
		{owner: model.PermissionAdmin, key: model.PermissionRead, created: true},
		{owner: model.PermissionAdmin, key: model.PermissionWrite, created: true},
	} {
		name := string(attempt.owner) + "-owner-" + string(attempt.key) + "-key"

		this.Equal(attempt.created, repo.Create(ctx, apiKeyRepositoryKey(name, owners[attempt.owner], attempt.key)), name)
		_, _, found := repo.FindByHash(ctx, []byte(name+"-hash"))
		this.Equal(attempt.created, found, name)
	}

	this.Equal(5, this.countRows("api_keys"))
}

func (this *IntegrationSuite) TestAPIKeyRepository_CreateForADeactivatedInvitedOrUnknownOwner_StoresNothing() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	deactivated := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	this.setUserState(deactivated, model.UserDeactivated)
	invited := this.apiKeyRepositoryCreateUser("bob@x.com", model.PermissionWrite)
	this.setUserState(invited, model.UserInvited)

	this.False(repo.Create(ctx, apiKeyRepositoryKey("ana-laptop", deactivated, model.PermissionRead)))
	this.False(repo.Create(ctx, apiKeyRepositoryKey("bob-laptop", invited, model.PermissionRead)))
	this.False(repo.Create(ctx, apiKeyRepositoryKey("nobody-laptop", deactivated+invited, model.PermissionRead)))
	this.Equal(0, this.countRows("api_keys"))
}

func (this *IntegrationSuite) TestAPIKeyRepository_CreateWhileADeactivationIsOpen_WaitsForItAndStoresNothing() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)

	deactivation, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = deactivation.Rollback(ctx) }()
	_, err = deactivation.Exec(ctx, `UPDATE users SET active = false, updated_at = now() WHERE id = $1`, userID)
	this.Require().NoError(err)

	var created bool
	createFinished := apiKeyRepositoryInBackground(func() {
		created = repo.Create(ctx, apiKeyRepositoryKey("ana-laptop", userID, model.PermissionRead))
	})
	this.apiKeyRepositoryAwaitLockWaiters(1)

	this.Require().NoError(deactivation.Commit(ctx))
	this.Nil(<-createFinished)
	this.False(created)
	this.Equal(0, this.countRows("api_keys"))
}

func (this *IntegrationSuite) TestAPIKeyRepository_CreateWhileADemotionIsOpen_WaitsForItAndStoresNothing() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)

	demotion, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = demotion.Rollback(ctx) }()
	_, err = demotion.Exec(ctx, `UPDATE users SET permission = 'read', updated_at = now() WHERE id = $1`, userID)
	this.Require().NoError(err)

	var created bool
	createFinished := apiKeyRepositoryInBackground(func() {
		created = repo.Create(ctx, apiKeyRepositoryKey("ana-laptop", userID, model.PermissionWrite))
	})
	this.apiKeyRepositoryAwaitLockWaiters(1)

	this.Require().NoError(demotion.Commit(ctx))
	this.Nil(<-createFinished)
	this.False(created)
	this.Equal(0, this.countRows("api_keys"))
}

func (this *IntegrationSuite) TestAPIKeyRepository_DeletingTheOwner_DeletesOnlyTheirKeys() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	ana := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	bob := this.apiKeyRepositoryCreateUser("bob@x.com", model.PermissionWrite)
	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("ana-laptop", ana, model.PermissionWrite)))
	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("ana-ci", ana, model.PermissionRead)))
	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("bob-laptop", bob, model.PermissionWrite)))

	_, err := this.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, ana)
	this.Require().NoError(err)

	this.Equal(1, this.countRows("api_keys"))
	_, _, found := repo.FindByHash(ctx, []byte("bob-laptop-hash"))
	this.True(found)
}

func (this *IntegrationSuite) TestAPIKeyRepository_ListByUser_PagesThatUsersKeysNewestFirstWithTheirTotal() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	ana := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	bob := this.apiKeyRepositoryCreateUser("bob@x.com", model.PermissionWrite)

	for _, created := range []struct {
		name    string
		daysAgo int
	}{
		{name: "ana-ci", daysAgo: 3},
		{name: "ana-laptop", daysAgo: 1},
		{name: "ana-phone", daysAgo: 2},
	} {
		this.Require().True(repo.Create(ctx, apiKeyRepositoryKey(created.name, ana, model.PermissionRead)))
		_, err := this.Pool.Exec(ctx,
			`UPDATE api_keys SET created_at = now() - make_interval(days => $2) WHERE name = $1`, created.name, created.daysAgo,
		)
		this.Require().NoError(err)
	}
	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("bob-laptop", bob, model.PermissionRead)))

	keys, total := repo.ListByUser(ctx, ana, 25, 0)
	this.Equal([]string{"ana-laptop", "ana-phone", "ana-ci"}, apiKeyRepositoryNamesOf(keys))
	this.Equal(3, total)

	keys, total = repo.ListByUser(ctx, ana, 2, 2)
	this.Equal([]string{"ana-ci"}, apiKeyRepositoryNamesOf(keys))
	this.Equal(3, total)

	keys, total = repo.ListByUser(ctx, ana, 2, 4)
	this.Empty(keys)
	this.Equal(3, total)

	keys, total = repo.ListByUser(ctx, this.adminID, 25, 0)
	this.Empty(keys)
	this.Equal(0, total)
}

func (this *IntegrationSuite) TestAPIKeyRepository_FindByHash_BringsTheOwnersCurrentPermission() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("ana-laptop", userID, model.PermissionWrite)))

	_, err := this.Pool.Exec(ctx, `UPDATE users SET permission = 'read' WHERE id = $1`, userID)
	this.Require().NoError(err)

	found, owner, ok := repo.FindByHash(ctx, []byte("ana-laptop-hash"))
	this.Require().True(ok)
	this.Equal(model.PermissionWrite, found.Permission)
	this.Equal(model.PermissionRead, owner.Permission)
}

func (this *IntegrationSuite) TestAPIKeyRepository_DeleteByIDAndUser_DeletesOnlyTheOwnersKeyAndReturnsItsName() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	ana := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	bob := this.apiKeyRepositoryCreateUser("bob@x.com", model.PermissionWrite)
	key := apiKeyRepositoryKey("ana-laptop", ana, model.PermissionWrite)
	this.Require().True(repo.Create(ctx, key))

	name, deleted := repo.DeleteByIDAndUser(ctx, key.ID, bob)
	this.False(deleted)
	this.Empty(name)
	_, _, found := repo.FindByHash(ctx, []byte("ana-laptop-hash"))
	this.True(found)

	name, deleted = repo.DeleteByIDAndUser(ctx, key.ID, ana)
	this.True(deleted)
	this.Equal("ana-laptop", name)
	_, _, found = repo.FindByHash(ctx, []byte("ana-laptop-hash"))
	this.False(found)

	_, deleted = repo.DeleteByIDAndUser(ctx, key.ID, ana)
	this.False(deleted)
}

func (this *IntegrationSuite) TestAPIKeyRepository_MarkUsed_SetsLastUsedAtOfThatKeyOnly() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	laptop := apiKeyRepositoryKey("ana-laptop", userID, model.PermissionWrite)
	this.Require().True(repo.Create(ctx, laptop))
	this.Require().True(repo.Create(ctx, apiKeyRepositoryKey("ana-ci", userID, model.PermissionRead)))

	repo.MarkUsed(ctx, laptop.ID)

	used, _, found := repo.FindByHash(ctx, []byte("ana-laptop-hash"))
	this.Require().True(found)
	this.True(used.LastUsedAt.Valid)
	this.WithinDuration(time.Now(), used.LastUsedAt.Time, time.Minute)

	unused, _, found := repo.FindByHash(ctx, []byte("ana-ci-hash"))
	this.Require().True(found)
	this.False(unused.LastUsedAt.Valid)
}

func (this *IntegrationSuite) TestAPIKeyRepository_MarkUsedWithinAMinuteOfTheLastUse_LeavesLastUsedAtUnchanged() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	key := apiKeyRepositoryKey("ana-laptop", userID, model.PermissionWrite)
	this.Require().True(repo.Create(ctx, key))
	_, err := this.Pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() - interval '30 seconds' WHERE id = $1`, key.ID)
	this.Require().NoError(err)
	lastUse := this.apiKeyRepositoryLastUsedAt(key.ID)

	repo.MarkUsed(ctx, key.ID)

	this.True(lastUse.Time.Equal(this.apiKeyRepositoryLastUsedAt(key.ID).Time))
}

func (this *IntegrationSuite) TestAPIKeyRepository_MarkUsedMoreThanAMinuteAfterTheLastUse_SetsLastUsedAtToNow() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)
	key := apiKeyRepositoryKey("ana-laptop", userID, model.PermissionWrite)
	this.Require().True(repo.Create(ctx, key))
	_, err := this.Pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() - interval '2 minutes' WHERE id = $1`, key.ID)
	this.Require().NoError(err)

	repo.MarkUsed(ctx, key.ID)

	this.WithinDuration(time.Now(), this.apiKeyRepositoryLastUsedAt(key.ID).Time, 5*time.Second)
}

func (this *IntegrationSuite) TestAPIKeyRepository_DeleteExpired_DeletesKeysPastTheirExpiryAndKeepsTheOthers() {
	ctx := context.Background()
	repo := repository.NewAPIKeyRepository(this.Pool)
	userID := this.apiKeyRepositoryCreateUser("ana@x.com", model.PermissionWrite)

	expired := apiKeyRepositoryKey("ana-expired", userID, model.PermissionRead)
	expired.ExpiresAt = null.TimeFrom(time.Now().Add(-time.Hour))
	fresh := apiKeyRepositoryKey("ana-fresh", userID, model.PermissionRead)
	fresh.ExpiresAt = null.TimeFrom(time.Now().Add(24 * time.Hour))
	never := apiKeyRepositoryKey("ana-never", userID, model.PermissionRead)
	for _, key := range []*model.APIKey{expired, fresh, never} {
		this.Require().True(repo.Create(ctx, key))
	}

	repo.DeleteExpired(ctx)

	this.Equal(2, this.countRows("api_keys"))
	_, _, found := repo.FindByHash(ctx, []byte("ana-expired-hash"))
	this.False(found)
	_, _, found = repo.FindByHash(ctx, []byte("ana-fresh-hash"))
	this.True(found)
	_, _, found = repo.FindByHash(ctx, []byte("ana-never-hash"))
	this.True(found)
}
