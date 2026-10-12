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
		cycle, found := cycleBackTo(declaration.Name, declaration.Schema, declarations.Catalog, contract.RefTrail{}.Enter(declaration.Name), map[string]bool{})
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

// cycleBackTo never explores a name twice: a name whose search did not lead back to start cannot lead there later,
// which keeps the search linear when schemas point at the next one in several fields.
func cycleBackTo(
	start string,
	schema contract.Schema,
	catalog contract.SchemasMap,
	trail contract.RefTrail,
	exploredWithoutCycle map[string]bool,
) (contract.RefTrail, bool) {
	switch {
	case schema.IsRef():
		if trail.Contains(schema.Ref) {
			return trail.Enter(schema.Ref), schema.Ref == start
		}

		target, declared := catalog[schema.Ref]
		if !declared || exploredWithoutCycle[schema.Ref] {
			return contract.RefTrail{}, false
		}

		cycle, found := cycleBackTo(start, target, catalog, trail.Enter(schema.Ref), exploredWithoutCycle)
		if !found {
			exploredWithoutCycle[schema.Ref] = true
		}

		return cycle, found

	case schema.IsArray():
		if schema.Items == nil {
			return contract.RefTrail{}, false
		}

		return cycleBackTo(start, *schema.Items, catalog, trail, exploredWithoutCycle)

	case schema.IsObject():
		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			if cycle, found := cycleBackTo(start, schema.Properties[name], catalog, trail, exploredWithoutCycle); found {
				return cycle, true
			}
		}

		if schema.Values != nil {
			return cycleBackTo(start, *schema.Values, catalog, trail, exploredWithoutCycle)
		}
	}

	return contract.RefTrail{}, false
}
