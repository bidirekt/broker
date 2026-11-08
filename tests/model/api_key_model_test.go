package model_test

import (
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

var keyExpiry = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func TestAPIKey_IsExpiredAt_NeverWithoutAnExpiry(t *testing.T) {
	key := &model.APIKey{}

	assert.False(t, key.IsExpiredAt(keyExpiry.AddDate(100, 0, 0)))
}

func TestAPIKey_IsExpiredAt_NotYetBeforeItsExpiry(t *testing.T) {
	key := &model.APIKey{ExpiresAt: null.TimeFrom(keyExpiry)}

	assert.False(t, key.IsExpiredAt(keyExpiry.Add(-time.Second)))
}

func TestAPIKey_IsExpiredAt_ExactlyAtItsExpiry(t *testing.T) {
	key := &model.APIKey{ExpiresAt: null.TimeFrom(keyExpiry)}

	assert.True(t, key.IsExpiredAt(keyExpiry))
}

func TestAPIKey_IsExpiredAt_AfterItsExpiry(t *testing.T) {
	key := &model.APIKey{ExpiresAt: null.TimeFrom(keyExpiry)}

	assert.True(t, key.IsExpiredAt(keyExpiry.Add(time.Second)))
}
