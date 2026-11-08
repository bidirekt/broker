package model

import (
	"slices"
	"time"

	"github.com/guregu/null"
)

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)

var permissionLadder = []Permission{PermissionRead, PermissionWrite, PermissionAdmin}

func (this Permission) Includes(needed Permission) bool {
	held, neededStep := slices.Index(permissionLadder, this), slices.Index(permissionLadder, needed)
	return neededStep >= 0 && held >= neededStep
}

func (this Permission) CappedBy(ceiling Permission) Permission {
	if ceiling.Includes(this) {
		return this
	}

	return ceiling
}

func (this Permission) IsGrantable() bool {
	return this == PermissionRead || this == PermissionWrite
}

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
