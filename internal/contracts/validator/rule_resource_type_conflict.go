package validator

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

type typedProperty struct {
	propertyType string
	source       string
}

// Only consumed resources merge, so only they can disagree with themselves.
func conflictingPropertyTypes(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	seen := map[string]map[string]typedProperty{}

	for _, declaration := range declarations.Resources {
		if !declaration.Resource.IsConsumer() {
			continue
		}

		if _, declared := declarations.Catalog[declaration.SchemaName]; !declared {
			continue
		}

		path := declaration.Path.String()

		declaredTypes, tracked := seen[path]
		if !tracked {
			declaredTypes = map[string]typedProperty{}
			seen[path] = declaredTypes
		}

		properties := declaration.Resource.Properties
		for _, propertyPath := range slices.Sorted(maps.Keys(properties)) {
			current := typedProperty{propertyType: properties[propertyPath].Type, source: declaration.Source}

			previous, taken := declaredTypes[propertyPath]
			if !taken {
				declaredTypes[propertyPath] = current

				continue
			}

			if previous.propertyType == current.propertyType {
				continue
			}

			violations = append(violations, violation.Violation{
				ErrorCode: "resource.type_conflict",
				Path:      path,
				Source:    declaration.Source,
				Details: map[string]string{
					"resource":     declaration.Resource.Describe(),
					"property":     propertyPath,
					"type":         current.propertyType,
					"declaredIn":   previous.source,
					"declaredType": previous.propertyType,
				},
			})
		}
	}

	return violations
}
