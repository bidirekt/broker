package validator

import (
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

type rule func(declarations protocol.Declarations) []violation.Violation

var rules = []rule{
	duplicateResources,
	conflictingPropertyTypes,
	duplicateSchemas,
	unresolvedSchemaNames,
	unresolvedSchemaRefs,
}

func schemaPath(name string) string {
	return "schemas;" + name
}
