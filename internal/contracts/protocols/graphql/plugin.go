package graphql

import (
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

type Plugin struct{}

func New() Plugin {
	return Plugin{}
}

func (this Plugin) Protocol() model.Protocol {
	return graphqlmodel.Protocol
}

func (this Plugin) Grammar() protocol.Grammar {
	block := descriptor.Object(descriptor.Fields{})

	return protocol.Grammar{Provides: block, Consumes: block}
}

func (this Plugin) ToDeclarations(protocol.Block) protocol.BlockDeclarations {
	return protocol.BlockDeclarations{}
}

func (this Plugin) NewReportTree() compatibility_checker.ReportTree {
	return graphqlmodel.ReportTree{}
}
