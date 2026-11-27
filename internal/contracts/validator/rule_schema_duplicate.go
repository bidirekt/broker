package validator

import (
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func duplicateSchemas(declarations protocol.Declarations) []violation.Violation {
	var violations []violation.Violation

	declaredIn := map[[2]string]string{}

	for _, declaration := range declarations.Schemas {
		key := [2]string{declaration.Namespace, declaration.Name}
		first, taken := declaredIn[key]
		if !taken {
			declaredIn[key] = declaration.Source

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
