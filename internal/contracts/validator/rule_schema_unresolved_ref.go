package validator

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

// Each schema is inspected in its own definition only, so a broken ref is reported once, where it is written.
func unresolvedSchemaRefs(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, declaration := range declarations.Schemas {
		violations = append(violations, unresolvedRefsIn(
			declaration.Schema,
			declarations.Catalog,
			declaration,
			schemaPath(declaration.Name),
			"",
		)...)
	}

	return violations
}

func unresolvedRefsIn(
	schema contract.Schema,
	catalog contract.SchemasMap,
	declaration fragmentmapper.SchemaDeclaration,
	documentPath string,
	path string,
) []violation.Violation {
	switch {
	case schema.IsRef():
		if _, declared := catalog[schema.Ref]; declared {
			return nil
		}

		return []violation.Violation{{
			ErrorCode: "schema.unresolved_ref",
			Path:      documentPath,
			Source:    declaration.Source,
			Details: map[string]string{
				"schema":   schema.Ref,
				"property": model.FieldName(declaration.Name, path),
			},
		}}

	case schema.IsArray():
		if schema.Items == nil {
			return nil
		}

		return unresolvedRefsIn(*schema.Items, catalog, declaration, documentPath+";items", model.ChildPath(path, model.ItemsSegment))

	case schema.IsObject():
		var violations []violation.Violation

		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			violations = append(violations, unresolvedRefsIn(
				schema.Properties[name],
				catalog,
				declaration,
				documentPath+";properties;"+name,
				model.ChildPath(path, name),
			)...)
		}

		return violations

	case schema.IsMap():
		if schema.Values == nil {
			return nil
		}

		return unresolvedRefsIn(*schema.Values, catalog, declaration, documentPath+";values", model.ChildPath(path, model.ValuesSegment))
	}

	return nil
}
