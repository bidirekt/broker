package fragmentmapper

import (
	"github.com/bidirekt/broker/internal/model"
)

// A provider declared twice is a violation the validator reports before this runs, so the first one wins here.
func mergeByResourcePath(declarations []ResourceDeclaration) []ResourceDeclaration {
	merged := make([]ResourceDeclaration, 0, len(declarations))
	indexByPath := make(map[string]int, len(declarations))

	for _, declaration := range declarations {
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

		merged[index].Resource = unionResourceModels(merged[index].Resource, declaration.Resource)
	}

	return merged
}

func unionResourceModels(a, b model.UploadedResource) model.UploadedResource {
	union := a

	switch a.Interaction {
	case model.RestRequest:
		union.Properties = unionRequestProperties(a.Properties, b.Properties)
	case model.RestResponse:
		union.Properties = unionResponseProperties(a.Properties, b.Properties)
	}

	return union
}

// What two senders send: a path is required only if both require it.
func unionRequestProperties(a, b map[string]model.Property) map[string]model.Property {
	union := make(map[string]model.Property, len(a)+len(b))

	for path, property := range a {
		other, declared := b[path]
		property.Optional = !declared || property.Optional || other.Optional
		union[path] = property
	}

	for path, property := range b {
		if _, declared := a[path]; declared {
			continue
		}

		property.Optional = true
		union[path] = property
	}

	return union
}

// What two readers need: a path is optional only if every reader that mentions it allows it.
func unionResponseProperties(a, b map[string]model.Property) map[string]model.Property {
	union := make(map[string]model.Property, len(a)+len(b))

	for path, property := range a {
		if other, declared := b[path]; declared {
			property.Optional = property.Optional && other.Optional
		}

		union[path] = property
	}

	for path, property := range b {
		if _, declared := a[path]; declared {
			continue
		}

		union[path] = property
	}

	return union
}
