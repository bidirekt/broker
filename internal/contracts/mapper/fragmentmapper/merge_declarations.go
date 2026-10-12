package fragmentmapper

// A provider declared twice is a violation the validator reports before this runs, so the first one wins here.
func ToMergedResourceDeclarations(declarations Declarations) []ResourceDeclaration {
	merged := make([]ResourceDeclaration, 0, len(declarations.Resources))
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
