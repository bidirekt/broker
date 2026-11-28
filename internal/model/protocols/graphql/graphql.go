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

// ReasonProviderGraphNotComposable is the break of every field a consumer reads from a graph that does not compose.
const ReasonProviderGraphNotComposable model.BreakingReason = "provider_graph_not_composable"

const (
	// CauseKey is the detail with the first error of a graph that does not compose.
	CauseKey = "cause"
	// CausesKey is the detail with every error of a graph that does not compose, one per line.
	CausesKey = "causes"
)
