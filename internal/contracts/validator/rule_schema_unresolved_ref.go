package validator

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

// Each schema is inspected in its own definition only, so a broken ref is reported once, where it is written.
func unresolvedSchemaRefs(declarations protocol.Declarations) []violation.Violation {
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
	declaration protocol.SchemaDeclaration,
	documentPath string,
	path string,
) []violation.Violation {
	if schema.IsRef() {
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
	}

	var violations []violation.Violation

	for _, child := range schema.Children() {
		violations = append(violations, unresolvedRefsIn(
			child.Schema,
			catalog,
			declaration,
			documentPath+";"+child.DocumentPath,
			model.ChildPath(path, child.Segment),
		)...)
	}

	return violations
}
