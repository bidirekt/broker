package graphql

import (
	"cmp"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// The type info and the directives are sorted, so the same SDL split into other files, or with its directives in
// another order, merges into the same extension. A contract belongs to one graph: the first one declared.
func (this Plugin) MergeExtensions(pieces []protocol.ExtensionPiece) (any, []violation.Violation) {
	var merged graphqlmodel.Extension
	var violations []violation.Violation

	for _, piece := range pieces {
		extension := piece.Value.(graphqlmodel.Extension)

		switch {
		case extension.Graph == "":
		case merged.Graph == "":
			merged.Graph, merged.GraphSource = extension.Graph, piece.Source
		case extension.Graph != merged.Graph:
			violations = append(violations, violation.Violation{
				ErrorCode: "graph.conflict",
				Path:      graphqlmodel.GraphPath,
				Source:    piece.Source,
				Details:   map[string]any{"graph": extension.Graph, "declaredIn": merged.GraphSource, "declared": merged.Graph},
			})
		}

		merged.Types = append(merged.Types, extension.Types...)
		merged.Directives = append(merged.Directives, extension.Directives...)

		merged.Federation1 = merged.Federation1 || extension.Federation1
		merged.LinkSource = cmp.Or(merged.LinkSource, extension.LinkSource)
		merged.Roots.Query = cmp.Or(merged.Roots.Query, extension.Roots.Query)
		merged.Roots.Mutation = cmp.Or(merged.Roots.Mutation, extension.Roots.Mutation)
	}

	slices.SortFunc(merged.Types, func(a, b graphqlmodel.TypeInfo) int {
		return cmp.Compare(a.Coordinate, b.Coordinate)
	})

	// One resolvable @key of a field set makes the type resolvable by it, whatever the order of its repetitions.
	slices.SortFunc(merged.Directives, func(a, b graphqlmodel.Directive) int {
		return cmp.Or(compareDirectives(a, b), compareResolvableFirst(a, b))
	})
	merged.Directives = slices.CompactFunc(merged.Directives, func(a, b graphqlmodel.Directive) bool {
		return compareDirectives(a, b) == 0
	})

	return merged, violations
}

func compareDirectives(a, b graphqlmodel.Directive) int {
	return cmp.Or(
		cmp.Compare(a.Coordinate, b.Coordinate),
		cmp.Compare(a.Directive, b.Directive),
		cmp.Compare(a.Fields, b.Fields),
		cmp.Compare(a.From, b.From),
	)
}

func compareResolvableFirst(a, b graphqlmodel.Directive) int {
	switch {
	case a.Resolvable == b.Resolvable:
		return 0
	case a.Resolvable:
		return -1
	default:
		return 1
	}
}
