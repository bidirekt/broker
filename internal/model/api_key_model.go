package model

import (
	"time"

	"github.com/guregu/null"
)

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
