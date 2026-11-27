package graphql

import "github.com/bidirekt/broker/internal/model"

const Protocol model.Protocol = "graphql"

const ReportKey = "operations"

const Namespace = "graphql"

// GeneratedNamespace holds the schemas the YAML grammar implies, which no YAML or SDL writes.
const GeneratedNamespace = "graphql_generated"

const EmptyArgumentsSchema = "EmptyArguments"

const (
	QueryOperation    = "query"
	MutationOperation = "mutation"
)

const (
	ArgumentsInteraction = "arguments"
	ReturnsInteraction   = "returns"
)

const GraphKey = "graph"

// GraphPath is where a subgraph declares its graph: provides.graphql.graph.
const GraphPath = "provides;graphql;graph"

const (
	OperationKey   = "operation"
	FieldKey       = "field"
	InteractionKey = "interaction"
)
