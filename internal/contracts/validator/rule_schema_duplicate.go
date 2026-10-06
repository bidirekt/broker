package validator

import (
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func duplicateSchemas(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	declaredIn := map[string]string{}

	for _, declaration := range declarations.Schemas {
		first, taken := declaredIn[declaration.Name]
		if !taken {
			declaredIn[declaration.Name] = declaration.Source

			continue
		}

		violations = append(violations, violation.Violation{
			ErrorCode: "schema.duplicate",
			Path:      schemaPath(declaration.Name),
			Source:    declaration.Source,
			Details: map[string]string{
				"schema":     declaration.Name,
				"declaredIn": first,
			},
		})
	}

	return violations
}
