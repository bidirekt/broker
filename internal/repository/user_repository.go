package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertInvitedUserQuery = `
		INSERT INTO users
			(email, permission)
		VALUES
			($1, $2)
		RETURNING id, active, created_at, updated_at
	`

	insertAdminQuery = `
		INSERT INTO users
			(email, permission, password_hash)
		VALUES
			($1, 'admin', $2)
		RETURNING id, permission, active, created_at, updated_at
	`

	findUserByIDQuery = `
		SELECT
			id, email, permission, password_hash, active, created_at, updated_at
		FROM
			users
		WHERE
			id = $1
	`

	findUserByEmailQuery = `
		SELECT
			id, email, permission, password_hash, active, created_at, updated_at
		FROM
			users
		WHERE
			lower(email) = lower($1)
	`

	adminExistsQuery = `
		SELECT EXISTS (
			SELECT
				1
			FROM
				users
			WHERE
				permission = 'admin'
		)
	`

	listUsersQuery = `
		SELECT
			id, email, permission, password_hash, active, created_at, updated_at
		FROM
			users
		ORDER BY
			email COLLATE "C"
		LIMIT $1 OFFSET $2
	`

	countUsersQuery = `
		SELECT
			count(*)
		FROM
			users
	`

	updateUserEmailAndPermissionQuery = `
		UPDATE users
		SET
			email = $2,
			permission = $3,
			updated_at = now()
		WHERE
			id = $1
	`

	deactivateUserQuery = `
		UPDATE users
		SET
			active = false,
			updated_at = now()
		WHERE
			id = $1 AND active
	`

	reactivateUserQuery = `
		UPDATE users
		SET
			active = true,
			updated_at = now()
		WHERE
			id = $1 AND NOT active
	`

	lockUserByLinkQuery = `
		SELECT
			u.id
		FROM
			users u
		JOIN
			one_time_links l ON l.user_id = u.id
		WHERE
			l.token_hash = $1
		FOR UPDATE OF u
	`

	useLinkQuery = `
		UPDATE one_time_links
		SET
			used_at = now()
		FROM
			users
		WHERE
			one_time_links.token_hash = $1
			AND one_time_links.used_at IS NULL
			AND one_time_links.expires_at > now()
			AND users.id = one_time_links.user_id
			AND users.active
		RETURNING one_time_links.user_id
	`

	setUserPasswordHashQuery = `
		UPDATE users
		SET
			password_hash = $2,
			updated_at = now()
		WHERE
			id = $1
	`

	changeVerifiedPasswordHashQuery = `
		UPDATE users
		SET
			password_hash = $3,
			updated_at = now()
		WHERE
			id = $1 AND active AND password_hash = $2
	`

	deleteUserSessionsQuery = `
		DELETE FROM sessions
		WHERE
			user_id = $1
	`

	deleteUserOtherSessionsQuery = `
		DELETE FROM sessions
		WHERE
			user_id = $1 AND id_hash <> $2
	`
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (this *UserRepository) CreateInvited(ctx context.Context, user *model.User, link *model.OneTimeLink) (created bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, insertInvitedUserQuery, user.Email, user.Permission).
		Scan(&user.ID, &user.Active, &user.CreatedAt, &user.UpdatedAt)
	if isUniqueViolation(err) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error inserting invited user: %w", err))
	}

	link.UserID = user.ID
	if err := tx.QueryRow(ctx, insertLinkQuery, link.TokenHash, link.UserID, link.Purpose, link.ExpiresAt).Scan(&link.ExpiresAt); err != nil {
		panic(fmt.Errorf("error inserting invite link: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true
}

func (this *UserRepository) CreateAdmin(ctx context.Context, admin *model.User, passwordHash string) (created bool) {
	err := this.pool.QueryRow(ctx, insertAdminQuery, admin.Email, passwordHash).
		Scan(&admin.ID, &admin.Permission, &admin.Active, &admin.CreatedAt, &admin.UpdatedAt)
	if isUniqueViolation(err) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error inserting the admin: %w", err))
	}

	return true
}

func (this *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, bool) {
	return this.findOne(ctx, findUserByIDQuery, id)
}

func (this *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, bool) {
	return this.findOne(ctx, findUserByEmailQuery, email)
}

func (this *UserRepository) findOne(ctx context.Context, query string, arg any) (*model.User, bool) {
	rows, err := this.pool.Query(ctx, query, arg)
	if err != nil {
		panic(fmt.Errorf("error finding user: %w", err))
	}

	user, err := pgx.CollectExactlyOneRow(rows, scanUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}

	if err != nil {
		panic(fmt.Errorf("error finding user: %w", err))
	}

	return user, true
}

func (this *UserRepository) AdminExists(ctx context.Context) bool {
	var exists bool
	if err := this.pool.QueryRow(ctx, adminExistsQuery).Scan(&exists); err != nil {
		panic(fmt.Errorf("error checking whether the admin exists: %w", err))
	}

	return exists
}

func (this *UserRepository) List(ctx context.Context, limit, offset int) (users []*model.User, total int) {
	tx, err := this.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, countUsersQuery).Scan(&total); err != nil {
		panic(fmt.Errorf("error counting users: %w", err))
	}

	rows, err := tx.Query(ctx, listUsersQuery, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing users: %w", err))
	}

	users, err = pgx.AppendRows(make([]*model.User, 0), rows, scanUser)
	if err != nil {
		panic(fmt.Errorf("error listing users: %w", err))
	}

	return users, total
}

func (this *UserRepository) UpdateEmailAndPermission(
	ctx context.Context,
	id int64,
	email string,
	permission model.Permission,
) (updated, conflict bool) {
	tag, err := this.pool.Exec(ctx, updateUserEmailAndPermissionQuery, id, email, permission)
	if isUniqueViolation(err) {
		return false, true
	}

	if err != nil {
		panic(fmt.Errorf("error updating user: %w", err))
	}

	return tag.RowsAffected() == 1, false
}

func (this *UserRepository) Deactivate(ctx context.Context, id int64) (changed bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, deactivateUserQuery, id)
	if err != nil {
		panic(fmt.Errorf("error deactivating user: %w", err))
	}

	if tag.RowsAffected() == 0 {
		return false
	}

	if _, err := tx.Exec(ctx, deleteUserSessionsQuery, id); err != nil {
		panic(fmt.Errorf("error deleting user sessions: %w", err))
	}

	if _, err := tx.Exec(ctx, deleteOpenLinkQuery, id); err != nil {
		panic(fmt.Errorf("error deleting user open link: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true
}

func (this *UserRepository) Reactivate(ctx context.Context, id int64) (changed bool) {
	tag, err := this.pool.Exec(ctx, reactivateUserQuery, id)
	if err != nil {
		panic(fmt.Errorf("error reactivating user: %w", err))
	}

	return tag.RowsAffected() == 1
}

func (this *UserRepository) SetPasswordWithLink(ctx context.Context, tokenHash []byte, passwordHash string) (set bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// the user row is locked before the link, as every writer of users, sessions and links does, so they never deadlock
	var userID int64
	err = tx.QueryRow(ctx, lockUserByLinkQuery, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error locking link user: %w", err))
	}

	err = tx.QueryRow(ctx, useLinkQuery, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}

	if err != nil {
		panic(fmt.Errorf("error using link: %w", err))
	}

	if _, err := tx.Exec(ctx, setUserPasswordHashQuery, userID, passwordHash); err != nil {
		panic(fmt.Errorf("error setting user password: %w", err))
	}

	if _, err := tx.Exec(ctx, deleteUserSessionsQuery, userID); err != nil {
		panic(fmt.Errorf("error deleting user sessions: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true
}

func (this *UserRepository) ChangePasswordKeepingSession(
	ctx context.Context,
	userID int64,
	verifiedPasswordHash string,
	newPasswordHash string,
	keptSessionIDHash []byte,
) (changed bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, changeVerifiedPasswordHashQuery, userID, verifiedPasswordHash, newPasswordHash)
	if err != nil {
		panic(fmt.Errorf("error changing user password: %w", err))
	}

	if tag.RowsAffected() == 0 {
		return false
	}

	if _, err := tx.Exec(ctx, deleteUserOtherSessionsQuery, userID, keptSessionIDHash); err != nil {
		panic(fmt.Errorf("error deleting user other sessions: %w", err))
	}

	if _, err := tx.Exec(ctx, deleteOpenLinkQuery, userID); err != nil {
		panic(fmt.Errorf("error deleting user open link: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return true
}

func scanUser(row pgx.CollectableRow) (*model.User, error) {
	user := &model.User{}
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Permission,
		&user.PasswordHash,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}
