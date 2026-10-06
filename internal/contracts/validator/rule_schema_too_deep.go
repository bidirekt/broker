package validator

import (
	"maps"
	"slices"
	"strconv"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func schemasTooDeep(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, declaration := range declarations.Schemas {
		if !exceedsDepth(declaration.Schema, declarations.Catalog, DepthCounter{}) {
			continue
		}

		violations = append(violations, violation.Violation{
			ErrorCode: "schema.too_deep",
			Path:      schemaPath(declaration.Name),
			Source:    declaration.Source,
			Details: map[string]string{
				"schema":   declaration.Name,
				"maxDepth": strconv.Itoa(schemamapper.MaxDepth),
			},
		})
	}

	return violations
}

func exceedsDepth(schema contract.Schema, catalog contract.SchemasMap, depth DepthCounter) bool {
	if depth.Exceeded() {
		return true
	}

	switch {
	case schema.IsRef():
		target, declared := catalog[schema.Ref]
		if !declared {
			return false
		}

		return exceedsDepth(target, catalog, depth.Deeper())

	case schema.IsArray():
		if schema.Items == nil {
			return false
		}

		return exceedsDepth(*schema.Items, catalog, depth.Deeper())

	case schema.IsObject():
		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			if exceedsDepth(schema.Properties[name], catalog, depth.Deeper()) {
				return true
			}
		}
	}

	return false
}
