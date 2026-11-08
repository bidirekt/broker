package model

import (
	"time"

	"github.com/guregu/null"
)

const APIKeyPrefix = "bdk_"

type APIKey struct {
	ID            int64
	UserID        int64
	KeyHash       []byte
	DisplayPrefix string
	Name          string
	Permission    Permission
	CreatedAt     time.Time
	LastUsedAt    null.Time
	ExpiresAt     null.Time
}

func (this *APIKey) IsExpiredAt(now time.Time) bool {
	return this.ExpiresAt.Valid && !now.Before(this.ExpiresAt.Time)
}
