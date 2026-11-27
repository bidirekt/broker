package validator

import (
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

// A cycle is reported once, at its first name, where that schema is written.
func refCycles(declarations protocol.Declarations) []violation.Violation {
	sources := map[[2]string]string{}
	for _, declaration := range declarations.Schemas {
		key := [2]string{declaration.Namespace, declaration.Name}
		if _, declared := sources[key]; !declared {
			sources[key] = declaration.Source
		}
	}

	var violations []violation.Violation

	for _, namespace := range slices.Sorted(maps.Keys(declarations.Catalog)) {
		for _, cycle := range refCyclesIn(declarations.Catalog[namespace]) {
			violations = append(violations, violation.Violation{
				ErrorCode: "schema.ref_cycle",
				Path:      schemaPath(cycle[0]),
				Source:    sources[[2]string{namespace, cycle[0]}],
				Details:   map[string]any{"schemas": cycle},
			})
		}
	}

	return violations
}

func refCyclesIn(catalog contract.SchemasMap) [][]string {
	var cycles [][]string

	walked := map[string]bool{}

	for _, start := range slices.Sorted(maps.Keys(catalog)) {
		var chain []string

		for current := start; !walked[current]; {
			schema, declared := catalog[current]
			if !declared || !schema.IsRef() {
				break
			}

			if at := slices.Index(chain, current); at != -1 {
				cycles = append(cycles, startingAtTheFirstName(chain[at:]))

				break
			}

			chain = append(chain, current)
			current = schema.Ref
		}

		for _, name := range chain {
			walked[name] = true
		}
	}

	return cycles
}

func startingAtTheFirstName(cycle []string) []string {
	first := slices.Index(cycle, slices.Min(cycle))

	return slices.Concat(cycle[first:], cycle[:first])
}
