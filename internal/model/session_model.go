package model

import "time"

type Session struct {
	IDHash     []byte
	UserID     int64
	CSRFToken  string
	CreatedAt  time.Time
	LastSeenAt time.Time
}
