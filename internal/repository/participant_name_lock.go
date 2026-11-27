package repository

import (
	"context"
	"fmt"
)

const lockParticipantNameQuery = `
	SELECT pg_advisory_xact_lock(hashtext('participant_name:' || $1))
`

// LockParticipantName serializes, until the transaction ends, everything that takes or checks the name; outside a
// transaction it holds only for the statement.
func LockParticipantName(ctx context.Context, q Querier, name string) {
	if _, err := q.Exec(ctx, lockParticipantNameQuery, name); err != nil {
		panic(fmt.Errorf("error locking participant name: %w", err))
	}
}
