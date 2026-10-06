package validator

import (
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

// Only a provider redeclaration is a violation: consumers merge by union in the mapper.
func duplicateResources(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	declaredIn := map[string]string{}

	for _, declaration := range declarations.Resources {
		if !declaration.Resource.IsProvider() {
			continue
		}

		path := declaration.Path.String()

		first, taken := declaredIn[path]
		if !taken {
			declaredIn[path] = declaration.Source

			continue
		}

		violations = append(violations, violation.Violation{
			ErrorCode: "resource.duplicate",
			Path:      path,
			Source:    declaration.Source,
			Details: map[string]string{
				"resource":   declaration.Resource.Describe(),
				"declaredIn": first,
			},
		})
	}

	return violations
}
