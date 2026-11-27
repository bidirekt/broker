package graphql

import (
	"context"
	"fmt"
	"net/url"

	"github.com/bidirekt/broker/internal/repository"
)

const deployedGraphsQuery = `
	SELECT
		e.name, gm.graph_name
	FROM
		current_deployments cd
	JOIN
		environments e ON e.id = cd.environment_id
	JOIN
		graphql_graph_members gm ON gm.contract_id = cd.contract_id
	WHERE
		cd.participant_id = $1
	ORDER BY
		e.name COLLATE "C"
`

// ParticipantLinks are the graph of each environment's deployed version of the participant.
func (this Plugin) ParticipantLinks(ctx context.Context, q repository.Querier, participantID int64) []repository.Link {
	rows, err := q.Query(ctx, deployedGraphsQuery, participantID)
	if err != nil {
		panic(fmt.Errorf("error listing deployed graphs: %w", err))
	}

	defer rows.Close()

	var links []repository.Link

	for rows.Next() {
		var environment, graph string
		if err := rows.Scan(&environment, &graph); err != nil {
			panic(fmt.Errorf("error scanning deployed graph: %w", err))
		}

		links = append(links, repository.Link{Label: "graph " + graph + " in " + environment, URL: graphURL(graph)})
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading deployed graphs: %w", err))
	}

	return links
}

func (this Plugin) CounterpartURL(ctx context.Context, q repository.Querier, name string) (string, bool) {
	if !NewGraphLoader(q).GraphNamed(ctx, name) {
		return "", false
	}

	return graphURL(name), true
}

func graphURL(graph string) string {
	return "/graphs/" + url.PathEscape(graph)
}
