package validator

import (
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

type rule func(declarations fragmentmapper.Declarations) []violation.Violation

var rules = []rule{
	duplicateResources,
	conflictingPropertyTypes,
	duplicateSchemas,
	unresolvedSchemaNames,
	unresolvedSchemaRefs,
	schemasTooDeep,
}

func schemaPath(name string) string {
	return "schemas;" + name
}
