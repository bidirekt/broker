package graphql

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/repository"
)

const (
	participantNamedQuery = `
		SELECT EXISTS(SELECT 1 FROM participants WHERE name = $1)
	`

	insertGraphMemberQuery = `
		INSERT INTO graphql_graph_members
			(contract_id, graph_name)
		VALUES
			($1, $2)
	`

	insertTypeInfoQuery = `
		INSERT INTO graphql_type_info
			(contract_id, coordinate, kind, named_type)
		SELECT
			$1, coordinate, kind, named_type
		FROM
			unnest($2::text[], $3::text[], $4::text[]) AS info (coordinate, kind, named_type)
	`

	insertDirectivesQuery = `
		INSERT INTO graphql_federation_directives
			(contract_id, coordinate, directive, fields, resolvable)
		SELECT
			$1, coordinate, directive, fields, resolvable
		FROM
			unnest($2::text[], $3::text[], $4::text[], $5::boolean[]) AS kept (coordinate, directive, fields, resolvable)
	`

	loadTypeInfoQuery = `
		SELECT
			contract_id,
			coordinate,
			kind,
			named_type
		FROM
			graphql_type_info
		WHERE
			contract_id = ANY($1)
		ORDER BY
			contract_id, coordinate COLLATE "C"
	`

	loadGraphMembersQuery = `
		SELECT
			contract_id,
			graph_name
		FROM
			graphql_graph_members
		WHERE
			contract_id = ANY($1)
	`

	loadDirectivesQuery = `
		SELECT
			contract_id,
			coordinate,
			directive,
			fields,
			resolvable
		FROM
			graphql_federation_directives
		WHERE
			contract_id = ANY($1)
		ORDER BY
			contract_id, coordinate COLLATE "C", directive COLLATE "C", fields COLLATE "C"
	`
)

// A graph takes a name participants share, so the name is locked until the publish ends.
func (this Plugin) RefuseContractExtension(ctx context.Context, q repository.Querier, extension any) (model.Refusal, bool) {
	graphqlExtension := extension.(graphqlmodel.Extension)
	if graphqlExtension.Graph == "" {
		return model.Refusal{}, false
	}

	repository.LockParticipantName(ctx, q, graphqlExtension.Graph)

	var taken bool
	if err := q.QueryRow(ctx, participantNamedQuery, graphqlExtension.Graph).Scan(&taken); err != nil {
		panic(fmt.Errorf("error checking graph name: %w", err))
	}

	if !taken {
		return model.Refusal{}, false
	}

	return model.Refusal{
		Code:    "graph.name_taken",
		Path:    graphqlmodel.GraphPath,
		Source:  graphqlExtension.GraphSource,
		Details: map[string]any{"graph": graphqlExtension.Graph},
	}, true
}

func (this Plugin) InsertContractExtension(ctx context.Context, q repository.Querier, contractID int64, extension any) {
	graphqlExtension := extension.(graphqlmodel.Extension)

	if graphqlExtension.Graph != "" {
		if _, err := q.Exec(ctx, insertGraphMemberQuery, contractID, graphqlExtension.Graph); err != nil {
			panic(fmt.Errorf("error inserting graphql graph member: %w", err))
		}
	}

	this.insertTypeInfo(ctx, q, contractID, graphqlExtension.Types)
	this.insertDirectives(ctx, q, contractID, graphqlExtension.Directives)
}

func (this Plugin) insertTypeInfo(ctx context.Context, q repository.Querier, contractID int64, types []graphqlmodel.TypeInfo) {
	if len(types) == 0 {
		return
	}

	coordinates := make([]string, 0, len(types))
	kinds := make([]string, 0, len(types))
	namedTypes := make([]string, 0, len(types))

	for _, info := range types {
		coordinates = append(coordinates, info.Coordinate)
		kinds = append(kinds, info.Kind)
		namedTypes = append(namedTypes, info.NamedType)
	}

	if _, err := q.Exec(ctx, insertTypeInfoQuery, contractID, coordinates, kinds, namedTypes); err != nil {
		panic(fmt.Errorf("error inserting graphql type info: %w", err))
	}
}

func (this Plugin) insertDirectives(ctx context.Context, q repository.Querier, contractID int64, directives []graphqlmodel.Directive) {
	if len(directives) == 0 {
		return
	}

	coordinates := make([]string, 0, len(directives))
	names := make([]string, 0, len(directives))
	fieldSets := make([]string, 0, len(directives))
	resolvables := make([]bool, 0, len(directives))

	for _, directive := range directives {
		coordinates = append(coordinates, directive.Coordinate)
		names = append(names, directive.Directive)
		fieldSets = append(fieldSets, directive.Fields)
		resolvables = append(resolvables, directive.Resolvable)
	}

	if _, err := q.Exec(ctx, insertDirectivesQuery, contractID, coordinates, names, fieldSets, resolvables); err != nil {
		panic(fmt.Errorf("error inserting graphql federation directives: %w", err))
	}
}

func (this Plugin) LoadContractExtensions(ctx context.Context, q repository.Querier, contractIDs []int64) map[int64]any {
	extensions := map[int64]*graphqlmodel.Extension{}
	extensionOf := func(contractID int64) *graphqlmodel.Extension {
		if extensions[contractID] == nil {
			extensions[contractID] = &graphqlmodel.Extension{}
		}

		return extensions[contractID]
	}

	this.loadGraphMembers(ctx, q, contractIDs, extensionOf)
	this.loadTypeInfo(ctx, q, contractIDs, extensionOf)
	this.loadDirectives(ctx, q, contractIDs, extensionOf)

	loaded := make(map[int64]any, len(extensions))
	for contractID, extension := range extensions {
		loaded[contractID] = *extension
	}

	return loaded
}

func (this Plugin) loadGraphMembers(ctx context.Context, q repository.Querier, contractIDs []int64, extensionOf func(int64) *graphqlmodel.Extension) {
	rows, err := q.Query(ctx, loadGraphMembersQuery, contractIDs)
	if err != nil {
		panic(fmt.Errorf("error loading graphql graph members: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var contractID int64
		var graph string

		if err := rows.Scan(&contractID, &graph); err != nil {
			panic(fmt.Errorf("error scanning graphql graph member: %w", err))
		}

		extensionOf(contractID).Graph = graph
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading graphql graph members: %w", err))
	}
}

func (this Plugin) loadTypeInfo(ctx context.Context, q repository.Querier, contractIDs []int64, extensionOf func(int64) *graphqlmodel.Extension) {
	rows, err := q.Query(ctx, loadTypeInfoQuery, contractIDs)
	if err != nil {
		panic(fmt.Errorf("error loading graphql type info: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var contractID int64
		var info graphqlmodel.TypeInfo

		if err := rows.Scan(&contractID, &info.Coordinate, &info.Kind, &info.NamedType); err != nil {
			panic(fmt.Errorf("error scanning graphql type info: %w", err))
		}

		extension := extensionOf(contractID)
		extension.Types = append(extension.Types, info)
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading graphql type info: %w", err))
	}
}

func (this Plugin) loadDirectives(ctx context.Context, q repository.Querier, contractIDs []int64, extensionOf func(int64) *graphqlmodel.Extension) {
	rows, err := q.Query(ctx, loadDirectivesQuery, contractIDs)
	if err != nil {
		panic(fmt.Errorf("error loading graphql federation directives: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var contractID int64
		var directive graphqlmodel.Directive

		if err := rows.Scan(&contractID, &directive.Coordinate, &directive.Directive, &directive.Fields, &directive.Resolvable); err != nil {
			panic(fmt.Errorf("error scanning graphql federation directive: %w", err))
		}

		extension := extensionOf(contractID)
		extension.Directives = append(extension.Directives, directive)
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading graphql federation directives: %w", err))
	}
}
