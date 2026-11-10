package rest

import (
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

func (this Plugin) ToResourceDeclarations(block protocol.Block) []protocol.ResourceDeclaration {
	var declarations []protocol.ResourceDeclaration

	for _, endpoint := range block.Document.Keys() {
		normalized := NormalizeEndpoint(endpoint)
		endpointPath := block.Path.Append(normalized)
		operations := block.Document.Mapping(endpoint)

		for _, method := range methods {
			operation := operations.Mapping(method.name)
			methodPath := endpointPath.Append(method.name)

			if request, present := operation["request"]; present {
				identity := restmodel.Identity{Endpoint: normalized, Method: method.name}
				declarations = append(declarations, block.ToResourceDeclaration(methodPath.Append("request"), model.Input, identity, request))
			}

			responses := operation.Mapping("responses")
			for _, status := range responses.Keys() {
				identity := restmodel.Identity{Endpoint: normalized, Method: method.name, Status: status}
				declarations = append(declarations, block.ToResourceDeclaration(methodPath.Append("responses", status), model.Output, identity, responses[status]))
			}
		}
	}

	return declarations
}
