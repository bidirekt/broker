package fragmentmapper

import "github.com/bidirekt/broker/internal/contracts/protocol"

// A provider declared twice is a violation the validator reports before this runs, so the first one wins here.
func ToMergedResourceDeclarations(declarations protocol.Declarations) []protocol.ResourceDeclaration {
	merged := make([]protocol.ResourceDeclaration, 0, len(declarations.Resources))
	indexByPath := make(map[string]int, len(declarations.Resources))

	for _, declaration := range declarations.Resources {
		key := declaration.Path.String()

		index, seen := indexByPath[key]
		if !seen {
			indexByPath[key] = len(merged)
			merged = append(merged, declaration)

			continue
		}

		if declaration.Resource.IsProvider() {
			continue
		}

		merged[index].Resource.Bodies = append(merged[index].Resource.Bodies, declaration.Resource.Bodies...)
	}

	return merged
}
