package graphql

import (
	"errors"
	"regexp"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/bidirekt/broker/internal/validations"
)

var fieldNamePattern = regexp.MustCompile(`^[_A-Za-z][_0-9A-Za-z]*$`)

// FieldName enters the resource hash, so a name no GraphQL field can have is refused, never normalized.
var FieldName = descriptor.Key{
	ErrorCode: "operation.name_syntax",
	Check:     checkFieldName,
	Details: func(key string) map[string]any {
		return map[string]any{"name": key}
	},
}

// A graph's name is one participants may not take, so it has their format.
var graphName = descriptor.Text{ErrorCode: "graph.name_syntax", Check: validations.ParticipantName, ValueDetail: graphqlmodel.GraphKey}

func (this Plugin) Grammar() protocol.Grammar {
	field := descriptor.Object(descriptor.Fields{
		graphqlmodel.ArgumentsInteraction: descriptor.SchemaRef,
		graphqlmodel.ReturnsInteraction:   descriptor.SchemaRef,
	}).Rules(returnsRequired)
	fields := descriptor.Map(FieldName, field)

	return protocol.Grammar{
		Provides: descriptor.Object(descriptor.Fields{graphqlmodel.GraphKey: graphName}),
		Consumes: descriptor.Object(descriptor.Fields{
			graphqlmodel.QueryOperation:    fields,
			graphqlmodel.MutationOperation: fields,
		}),
	}
}

func checkFieldName(key string) error {
	if !fieldNamePattern.MatchString(key) {
		return errors.New("must be a GraphQL name")
	}

	return nil
}

// Every field reads something, and its resource breaks are reported on what it returns.
func returnsRequired(present descriptor.PresentFields) *violation.Violation {
	if present[graphqlmodel.ReturnsInteraction] != nil {
		return nil
	}

	return &violation.Violation{ErrorCode: "returns.required", Details: map[string]any{}}
}
