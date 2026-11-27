package graphql

import "github.com/bidirekt/broker/internal/model"

const Protocol model.Protocol = "graphql"

const ReportKey = "operations"

const Namespace = "graphql"

const (
	QueryOperation    = "query"
	MutationOperation = "mutation"
)

const (
	ArgumentsInteraction = "arguments"
	ReturnsInteraction   = "returns"
)

const (
	OperationKey   = "operation"
	FieldKey       = "field"
	InteractionKey = "interaction"
)
