package graphql

import (
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
