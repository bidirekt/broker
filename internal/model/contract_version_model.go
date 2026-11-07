package model

import "time"

type ContractVersion struct {
	ID              int64
	ParticipantID   int64
	ParticipantName string
	Version         string
	ContractContent string
	CreatedAt       time.Time
}
