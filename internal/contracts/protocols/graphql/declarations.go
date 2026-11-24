package graphql

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	"github.com/guregu/null"
)

var operations = []string{graphqlmodel.QueryOperation, graphqlmodel.MutationOperation}

// A resource has one body, so a field is two: the arguments it sends and what it reads back.
func (this Plugin) ToDeclarations(block protocol.Block) protocol.BlockDeclarations {
	var declarations protocol.BlockDeclarations

	lacksArguments := false

	for _, operation := range operations {
		fields := block.Document.Mapping(operation)

		for _, field := range fields.Keys() {
			declared := fields.Mapping(field)
			fieldPath := block.Path.Append(operation, field)

			arguments := graphqlmodel.Identity{Operation: operation, Field: field, Interaction: graphqlmodel.ArgumentsInteraction}
			argumentsPath := fieldPath.Append(graphqlmodel.ArgumentsInteraction)

			if argumentsRef := declared[graphqlmodel.ArgumentsInteraction]; argumentsRef != nil {
				declarations.Resources = append(declarations.Resources, block.ToResourceDeclaration(argumentsPath, model.Input, arguments, argumentsRef))
			} else {
				declarations.Resources = append(declarations.Resources, toEmptyArgumentsDeclaration(block, argumentsPath, arguments))
				lacksArguments = true
			}

			returns := graphqlmodel.Identity{Operation: operation, Field: field, Interaction: graphqlmodel.ReturnsInteraction}
			declarations.Resources = append(declarations.Resources,
				block.ToResourceDeclaration(fieldPath.Append(graphqlmodel.ReturnsInteraction), model.Output, returns, declared[graphqlmodel.ReturnsInteraction]))
		}
	}

	if lacksArguments {
		declarations.Schemas = []protocol.SchemaDeclaration{{
			Source:    block.Source,
			Namespace: graphqlmodel.GeneratedNamespace,
			Name:      graphqlmodel.EmptyArgumentsSchema,
			Schema:    contract.Schema{Type: model.ObjectType},
		}}
	}

	return declarations
}

// No arguments means none are sent: an empty object, never `~`, which would leave the body optional to the sender.
func toEmptyArgumentsDeclaration(block protocol.Block, path contract.ResourcePath, identity graphqlmodel.Identity) protocol.ResourceDeclaration {
	return protocol.ResourceDeclaration{
		Source:     block.Source,
		Path:       path,
		Namespace:  graphqlmodel.GeneratedNamespace,
		SchemaName: graphqlmodel.EmptyArgumentsSchema,
		Resource: model.UploadedResource{Resource: model.Resource{
			Direction:        block.Direction,
			Protocol:         block.Protocol,
			Body:             model.Input,
			Identity:         identity,
			ConsumedProvider: block.ConsumedProvider,
			Bodies: []model.ResourceBody{{
				Source:     block.Source,
				Namespace:  graphqlmodel.GeneratedNamespace,
				SchemaName: null.StringFrom(graphqlmodel.EmptyArgumentsSchema),
			}},
		}},
	}
}
