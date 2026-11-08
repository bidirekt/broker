package access_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

func activeUserWith(permission model.Permission) *model.User {
	return &model.User{ID: 7, Email: "ana@x.com", Permission: permission, PasswordHash: null.StringFrom("hash"), Active: true}
}

func TestPrincipalOfAnActiveUserCarriesTheUserPermission(t *testing.T) {
	principal, ok := access.PrincipalOf(activeUserWith(model.PermissionAdmin))

	assert.True(t, ok)
	assert.Equal(t, access.Principal{UserID: 7, Email: "ana@x.com", Permission: model.PermissionAdmin}, principal)
}

func TestPrincipalCappedByTakesTheLowerOfCeilingAndUserPermission(t *testing.T) {
	for _, tc := range []struct {
		user, ceiling, expected model.Permission
	}{
		{model.PermissionWrite, model.PermissionRead, model.PermissionRead},
		{model.PermissionRead, model.PermissionWrite, model.PermissionRead},
		{model.PermissionAdmin, model.PermissionWrite, model.PermissionWrite},
		{model.PermissionWrite, model.PermissionWrite, model.PermissionWrite},
	} {
		principal, ok := access.PrincipalCappedBy(activeUserWith(tc.user), tc.ceiling)

		assert.True(t, ok)
		assert.Equal(t, tc.expected, principal.Permission, "user %s capped by %s", tc.user, tc.ceiling)
	}
}

func TestPrincipalCappedByRefusesAUserWhoIsNotActive(t *testing.T) {
	deactivated := activeUserWith(model.PermissionWrite)
	deactivated.Active = false
	invited := activeUserWith(model.PermissionWrite)
	invited.PasswordHash = null.String{}

	for _, user := range []*model.User{deactivated, invited} {
		_, ok := access.PrincipalCappedBy(user, model.PermissionWrite)
		assert.False(t, ok)
	}
}

func TestPrincipalCanWriteWithWriteOrAdmin(t *testing.T) {
	assert.False(t, access.Principal{Permission: model.PermissionRead}.CanWrite())
	assert.True(t, access.Principal{Permission: model.PermissionWrite}.CanWrite())
	assert.True(t, access.Principal{Permission: model.PermissionAdmin}.CanWrite())
}

func TestPrincipalWithoutPermissionCannotWrite(t *testing.T) {
	assert.False(t, access.Principal{}.CanWrite())
}
