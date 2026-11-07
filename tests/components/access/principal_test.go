package access_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPrincipalCanWriteWithWriteOrAdmin(t *testing.T) {
	assert.False(t, access.Principal{Permission: model.PermissionRead}.CanWrite())
	assert.True(t, access.Principal{Permission: model.PermissionWrite}.CanWrite())
	assert.True(t, access.Principal{Permission: model.PermissionAdmin}.CanWrite())
}

func TestPrincipalWithoutPermissionCannotWrite(t *testing.T) {
	assert.False(t, access.Principal{}.CanWrite())
}
