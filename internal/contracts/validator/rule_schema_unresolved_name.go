package validator

import (
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func unresolvedSchemaNames(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, declaration := range declarations.Resources {
		if _, declared := declarations.Catalog[declaration.SchemaName]; declared {
			continue
		}

		violations = append(violations, violation.Violation{
			ErrorCode: "schema.unresolved_name",
			Path:      declaration.Path.String(),
			Source:    declaration.Source,
			Details: map[string]string{
				"schema":   declaration.SchemaName,
				"resource": declaration.Resource.Describe(),
			},
		})
	}

	return violations
}
