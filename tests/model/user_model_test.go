package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPermission_Includes_ClimbsTheLadderReadWriteAdmin(t *testing.T) {
	for _, step := range []struct {
		held     model.Permission
		needed   model.Permission
		includes bool
	}{
		{model.PermissionRead, model.PermissionRead, true},
		{model.PermissionRead, model.PermissionWrite, false},
		{model.PermissionRead, model.PermissionAdmin, false},
		{model.PermissionWrite, model.PermissionRead, true},
		{model.PermissionWrite, model.PermissionWrite, true},
		{model.PermissionWrite, model.PermissionAdmin, false},
		{model.PermissionAdmin, model.PermissionRead, true},
		{model.PermissionAdmin, model.PermissionWrite, true},
		{model.PermissionAdmin, model.PermissionAdmin, true},
	} {
		t.Run(string(step.held)+" includes "+string(step.needed), func(t *testing.T) {
			assert.Equal(t, step.includes, step.held.Includes(step.needed))
		})
	}
}

func TestPermission_IsGrantable_OnlyForReadAndWrite(t *testing.T) {
	assert.True(t, model.PermissionRead.IsGrantable())
	assert.True(t, model.PermissionWrite.IsGrantable())
	assert.False(t, model.PermissionAdmin.IsGrantable())
	assert.False(t, model.Permission("Write").IsGrantable())
	assert.False(t, model.Permission("").IsGrantable())
}

func TestPermission_Includes_RefusesAnUnknownPermission(t *testing.T) {
	assert.False(t, model.PermissionAdmin.Includes("owner"))
	assert.False(t, model.Permission("owner").Includes(model.PermissionRead))
	assert.False(t, model.Permission("").Includes(""))
}

func TestPermission_CappedBy_DropsToALowerCeiling(t *testing.T) {
	assert.Equal(t, model.PermissionRead, model.PermissionWrite.CappedBy(model.PermissionRead))
}

func TestPermission_CappedBy_KeepsItselfUnderAnEqualCeiling(t *testing.T) {
	assert.Equal(t, model.PermissionWrite, model.PermissionWrite.CappedBy(model.PermissionWrite))
}

func TestPermission_CappedBy_KeepsItselfUnderAHigherCeiling(t *testing.T) {
	assert.Equal(t, model.PermissionWrite, model.PermissionWrite.CappedBy(model.PermissionAdmin))
}
