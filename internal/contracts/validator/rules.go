package validator

import "github.com/bidirekt/broker/internal/contracts/protocol"

var rules = []protocol.Rule{
	duplicateResources,
	conflictingPropertyTypes,
	duplicateSchemas,
	unresolvedSchemaNames,
	unresolvedSchemaRefs,
	refCycles,
}

func schemaPath(name string) string {
	return "schemas;" + name
}
