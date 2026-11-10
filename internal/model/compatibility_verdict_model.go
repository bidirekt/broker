package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// VerdictBreak is a stored property break. On the wire its location sits under its
// protocol's key: {"rest": {...}, "reason": ..., "details": ...}.
type VerdictBreak struct {
	Protocol Protocol
	Location map[string]string
	Reason   BreakingReason
	Details  map[string]string
}

const (
	verdictReasonKey  = "reason"
	verdictDetailsKey = "details"
)

func (this VerdictBreak) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		string(this.Protocol): this.Location,
		verdictReasonKey:      this.Reason,
		verdictDetailsKey:     this.Details,
	})
}

func (this *VerdictBreak) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	decoded := VerdictBreak{}

	for key, value := range fields {
		var err error

		switch key {
		case verdictReasonKey:
			err = json.Unmarshal(value, &decoded.Reason)
		case verdictDetailsKey:
			err = json.Unmarshal(value, &decoded.Details)
		default:
			if decoded.Protocol != "" {
				return fmt.Errorf("verdict break has more than one protocol: %q and %q", decoded.Protocol, key)
			}

			decoded.Protocol = Protocol(key)
			err = json.Unmarshal(value, &decoded.Location)
		}

		if err != nil {
			return err
		}
	}

	if decoded.Protocol == "" {
		return fmt.Errorf("verdict break has no protocol")
	}

	*this = decoded

	return nil
}

type CompatibilityVerdict struct {
	ContractIDOne int64
	ContractIDTwo int64
	Breaks        []VerdictBreak
	Deployable    bool
	CreatedAt     time.Time
}
