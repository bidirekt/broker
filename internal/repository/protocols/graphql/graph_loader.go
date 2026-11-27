package graphql

import (
	"context"
	"fmt"

	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	deployedMembersQuery = `
		SELECT cd.participant_id, cd.contract_id, cd.version
		FROM current_deployments cd
		JOIN graphql_graph_members gm ON gm.contract_id = cd.contract_id
		WHERE cd.environment_id = $1 AND gm.graph_name = $2
	`

	// A participant that once consumed the provider is listed too; what its deployed contract consumes decides.
	deployedConsumersQuery = `
		SELECT cd.participant_id, cd.contract_id, cd.version
		FROM current_deployments cd
		WHERE cd.environment_id = $1
		  AND EXISTS (
			SELECT 1 FROM resources r
			WHERE r.participant_id = cd.participant_id
			  AND r.direction = 'consumes'
			  AND r.protocol = 'graphql'
			  AND r.consumed_provider = $2
		  )
		ORDER BY cd.participant_id
	`

	deployedGraphQuery = `
		SELECT COALESCE((
			SELECT gm.graph_name
			FROM current_deployments cd
			JOIN graphql_graph_members gm ON gm.contract_id = cd.contract_id
			WHERE cd.environment_id = $1 AND cd.participant_id = $2
		), '')
	`
)

type GraphLoader struct {
	q repository.Querier
}

func NewGraphLoader(q repository.Querier) GraphLoader {
	return GraphLoader{q: q}
}

func (this GraphLoader) GraphNamed(ctx context.Context, name string) bool {
	var named bool
	if err := this.q.QueryRow(ctx, graphNamedQuery, name).Scan(&named); err != nil {
		panic(fmt.Errorf("error checking graph name: %w", err))
	}

	return named
}

func (this GraphLoader) DeployedMembers(ctx context.Context, environmentID int64, graph string) []graphqlmodel.DeployedContract {
	return this.deployedContracts(ctx, deployedMembersQuery, environmentID, graph)
}

func (this GraphLoader) DeployedConsumers(ctx context.Context, environmentID int64, provider string) []graphqlmodel.DeployedContract {
	return this.deployedContracts(ctx, deployedConsumersQuery, environmentID, provider)
}

// DeployedGraph is the graph of the version the participant has deployed in the environment, empty when there is none.
func (this GraphLoader) DeployedGraph(ctx context.Context, environmentID, participantID int64) string {
	var graph string

	if err := this.q.QueryRow(ctx, deployedGraphQuery, environmentID, participantID).Scan(&graph); err != nil {
		panic(fmt.Errorf("error loading deployed graph: %w", err))
	}

	return graph
}

func (this GraphLoader) deployedContracts(ctx context.Context, query string, environmentID int64, of any) []graphqlmodel.DeployedContract {
	rows, err := this.q.Query(ctx, query, environmentID, of)
	if err != nil {
		panic(fmt.Errorf("error loading deployed contracts of %v: %w", of, err))
	}
	defer rows.Close()

	var deployed []graphqlmodel.DeployedContract

	for rows.Next() {
		var contract graphqlmodel.DeployedContract
		if err := rows.Scan(&contract.ParticipantID, &contract.ContractID, &contract.Version); err != nil {
			panic(fmt.Errorf("error scanning deployed contract of %v: %w", of, err))
		}

		deployed = append(deployed, contract)
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading deployed contracts of %v: %w", of, err))
	}

	return deployed
}
