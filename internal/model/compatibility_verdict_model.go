package model

import "time"

type VerdictBreak struct {
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Interaction string            `json:"interaction"`
	Reason      BreakingReason    `json:"reason"`
	Details     map[string]string `json:"details"`
}

type CompatibilityVerdict struct {
	ContractIDOne int64
	ContractIDTwo int64
	Breaks        []VerdictBreak
	Deployable    bool
	CreatedAt     time.Time
}
