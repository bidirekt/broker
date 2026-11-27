package graphql

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/repository"
)

const graphNamedQuery = `
	SELECT EXISTS(SELECT 1 FROM graphql_graph_members WHERE graph_name = $1)
`

func (this Plugin) ReservedParticipantName(ctx context.Context, q repository.Querier, name string) (string, bool) {
	var reserved bool
	if err := q.QueryRow(ctx, graphNamedQuery, name).Scan(&reserved); err != nil {
		panic(fmt.Errorf("error checking graph name: %w", err))
	}

	if !reserved {
		return "", false
	}

	return fmt.Sprintf("%q is a graph name; pick another participant name", name), true
}
