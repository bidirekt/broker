package repository

import "context"

// ReservedNames are names a protocol holds that no participant may take; it panics on a database error.
type ReservedNames interface {
	ReservedParticipantName(ctx context.Context, q Querier, name string) (message string, reserved bool)
}
