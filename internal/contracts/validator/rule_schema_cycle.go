package validator

import (
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func schemasInCycle(declarations fragmentmapper.Declarations) []violation.Violation {
	var violations []violation.Violation

	for _, declaration := range declarations.Schemas {
		cycle, found := cycleBackTo(declaration.Name, declaration.Schema, declarations.Catalog, contract.RefTrail{}.Enter(declaration.Name))
		if !found {
			continue
		}

		violations = append(violations, violation.Violation{
			ErrorCode: "schema.cycle",
			Path:      schemaPath(declaration.Name),
			Source:    declaration.Source,
			Details: map[string]string{
				"schema": declaration.Name,
				"cycle":  strings.Join(cycle.Names(), " → "),
			},
		})
	}

	return violations
}

func cycleBackTo(start string, schema contract.Schema, catalog contract.SchemasMap, trail contract.RefTrail) (contract.RefTrail, bool) {
	switch {
	case schema.IsRef():
		if trail.Contains(schema.Ref) {
			return trail.Enter(schema.Ref), schema.Ref == start
		}

		target, declared := catalog[schema.Ref]
		if !declared {
			return contract.RefTrail{}, false
		}

		return cycleBackTo(start, target, catalog, trail.Enter(schema.Ref))

	case schema.IsArray():
		if schema.Items == nil {
			return contract.RefTrail{}, false
		}

		return cycleBackTo(start, *schema.Items, catalog, trail)

	case schema.IsObject():
		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			if cycle, found := cycleBackTo(start, schema.Properties[name], catalog, trail); found {
				return cycle, true
			}
		}
	}

	return contract.RefTrail{}, false
}
