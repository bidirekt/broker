package model

import (
	"time"

	"github.com/guregu/null"
)

type LinkPurpose string

const (
	LinkPurposeInvite LinkPurpose = "invite"
	LinkPurposeReset  LinkPurpose = "reset"
)

type OneTimeLink struct {
	TokenHash []byte
	UserID    int64
	Purpose   LinkPurpose
	ExpiresAt time.Time
	UsedAt    null.Time
}
