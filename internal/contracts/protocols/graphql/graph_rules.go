package graphql

import (
	"slices"

	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/violation"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

// A graph is composed from its subgraphs' SDL, so a graph: needs the SDL in the same publish.
func graphWithoutSchema(declarations protocol.Declarations) []violation.Violation {
	extension, _ := declarations.Extensions[graphqlmodel.Protocol.String()].(graphqlmodel.Extension)
	if extension.Graph == "" || slices.Contains(declarations.Formats, formatName) {
		return nil
	}

	return []violation.Violation{{
		ErrorCode: "graph.without_schema",
		Path:      graphqlmodel.GraphPath,
		Source:    extension.GraphSource,
		Details:   map[string]any{"graph": extension.Graph},
	}}
}

// A federated SDL is a subgraph, and a subgraph belongs to a graph.
func federationWithoutGraph(declarations protocol.Declarations) []violation.Violation {
	extension, _ := declarations.Extensions[graphqlmodel.Protocol.String()].(graphqlmodel.Extension)
	if extension.LinkSource == "" || extension.Graph != "" {
		return nil
	}

	return []violation.Violation{{
		ErrorCode: "graph.required_by_federation",
		Path:      "",
		Source:    extension.LinkSource,
		Details:   map[string]any{},
	}}
}
