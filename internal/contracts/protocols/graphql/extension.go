package graphql

import (
	"cmp"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// The type info is sorted by coordinate, so the same SDL split into other files merges into the same extension.
func (this Plugin) MergeExtensions(pieces []protocol.ExtensionPiece) (any, []violation.Violation) {
	var merged graphqlmodel.Extension

	for _, piece := range pieces {
		merged.Types = append(merged.Types, piece.Value.(graphqlmodel.Extension).Types...)
	}

	slices.SortFunc(merged.Types, func(a, b graphqlmodel.TypeInfo) int {
		return cmp.Compare(a.Coordinate, b.Coordinate)
	})

	return merged, nil
}
