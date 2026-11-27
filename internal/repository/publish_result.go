package repository

import "github.com/bidirekt/broker/internal/model"

type PublishOutcome int

const (
	Published PublishOutcome = iota
	PublishConflicted
	PublishRefused
)

// PublishResult carries the Refusal when the Outcome is PublishRefused.
type PublishResult struct {
	Outcome PublishOutcome
	Refusal model.Refusal
}
