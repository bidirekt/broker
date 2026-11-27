package graphql

import (
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

const (
	scalarNameKey = "name"
	argumentsKey  = "arguments"
)

func (this Plugin) SchemaVocabulary() descriptor.Vocabulary {
	return descriptor.Vocabulary{
		Types: []descriptor.TypeVocabulary{
			{
				Name:  model.ScalarType,
				Keys:  descriptor.Keys{scalarNameKey: func(descriptor.Vocabulary) descriptor.Node { return descriptor.String() }},
				Rules: []descriptor.Rule{scalarRequiresName},
			},
			{Name: model.UnionType, Takes: []string{"properties"}},
		},
		PropertyKeys: descriptor.Keys{argumentsKey: func(descriptor.Vocabulary) descriptor.Node { return descriptor.SchemaRef }},
	}
}

func scalarRequiresName(present descriptor.PresentFields) *violation.Violation {
	if present["ref"] != nil || present["type"] != model.ScalarType || (present[scalarNameKey] != nil && present[scalarNameKey] != "") {
		return nil
	}

	return &violation.Violation{ErrorCode: "schema.scalar_without_name"}
}
