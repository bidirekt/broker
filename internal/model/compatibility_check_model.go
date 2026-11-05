package model

import (
	"encoding/json"
	"time"

	"github.com/guregu/null"
)

type CompatibilityCheck struct {
	ID              int64
	ParticipantID   int64
	ContractID      int64
	Version         string
	EnvironmentID   int64
	EnvironmentName string
	Deployable      bool
	Report          json.RawMessage
	CreatedAt       time.Time
}

type CompatibilityCheckResult struct {
	ID                       int64
	CheckID                  int64
	CounterpartName          string
	CounterpartParticipantID null.Int
	CounterpartVersion       null.String
	VerdictContractIDOne     null.Int
	VerdictContractIDTwo     null.Int
	Deployable               bool
	Role                     CounterpartRole
}
