package model

import (
	"time"

	"github.com/guregu/null"
)

type Session struct {
	IDHash     []byte
	UserID     null.Int
	CSRFToken  string
	CreatedAt  time.Time
	LastSeenAt time.Time
}
