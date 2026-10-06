package validator

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

// Each schema is inspected in its own definition only, so a broken ref is reported once, where it is written.
func unresolvedSchemaRefs(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, declaration := range declarations.Schemas {
		violations = append(violations, unresolvedRefsIn(
			declaration.Schema,
			declarations.Catalog,
			declaration.Source,
			schemaPath(declaration.Name),
			declaration.Name,
		)...)
	}

	return violations
}

func unresolvedRefsIn(schema contract.Schema, catalog contract.SchemasMap, source string, path string, property string) []violation.Violation {
	switch {
	case schema.IsRef():
		if _, declared := catalog[schema.Ref]; declared {
			return nil
		}

		return []violation.Violation{{
			ErrorCode: "schema.unresolved_ref",
			Path:      path,
			Source:    source,
			Details: map[string]string{
				"schema":   schema.Ref,
				"property": property,
			},
		}}

	case schema.IsArray():
		if schema.Items == nil {
			return nil
		}

		return unresolvedRefsIn(*schema.Items, catalog, source, path+";items", property+"[]")

	case schema.IsObject():
		var violations []violation.Violation

		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			violations = append(violations, unresolvedRefsIn(
				schema.Properties[name],
				catalog,
				source,
				path+";properties;"+name,
				property+"."+name,
			)...)
		}

		return violations
	}

	return nil
}
