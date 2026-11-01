package model

import (
	"time"

	"github.com/guregu/null"
)

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
)

type UserState string

const (
	UserInvited     UserState = "invited"
	UserActive      UserState = "active"
	UserDeactivated UserState = "deactivated"
)

type User struct {
	ID           int64
	Email        string
	Permission   Permission
	PasswordHash null.String
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (this *User) State() UserState {
	if !this.Active {
		return UserDeactivated
	}

	if !this.PasswordHash.Valid {
		return UserInvited
	}

	return UserActive
}
