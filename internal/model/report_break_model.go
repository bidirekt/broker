package model

type ReportBreak struct {
	Reason  BreakingReason    `json:"reason"`
	Role    BreakRole         `json:"role"`
	Details map[string]string `json:"details,omitempty"`
}
