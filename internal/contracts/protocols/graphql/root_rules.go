package graphql

import (
	"slices"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// A graph names every subgraph's roots Query and Mutation, so a type of that name that is no root would take it.
func rootNameTaken(declarations protocol.Declarations) []violation.Violation {
	extension, _ := declarations.Extensions[graphqlmodel.Protocol.String()].(graphqlmodel.Extension)
	roots := []string{extension.Roots.Query, extension.Roots.Mutation}

	var violations []violation.Violation
	for _, root := range []struct{ operation, declared, typeName string }{
		{graphqlmodel.QueryOperation, extension.Roots.Query, "Query"},
		{graphqlmodel.MutationOperation, extension.Roots.Mutation, "Mutation"},
	} {
		if root.declared == "" || slices.Contains(roots, root.typeName) {
			continue
		}

		if source, declared := typeSourceOf(declarations.ExtensionPieces[graphqlmodel.Protocol.String()], root.typeName); declared {
			violations = append(violations, violation.Violation{
				ErrorCode: "schema.root_name_taken",
				Path:      "",
				Source:    source,
				Details:   map[string]any{"operation": root.operation, "root": root.declared, "type": root.typeName},
			})
		}
	}

	return violations
}

func typeSourceOf(pieces []protocol.ExtensionPiece, typeName string) (string, bool) {
	for _, piece := range pieces {
		for _, info := range piece.Value.(graphqlmodel.Extension).Types {
			if info.Coordinate == typeName {
				return piece.Source, true
			}
		}
	}

	return "", false
}
