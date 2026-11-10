package rest

import (
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

type Plugin struct{}

func New() Plugin {
	return Plugin{}
}

func (this Plugin) Protocol() model.Protocol {
	return restmodel.Protocol
}
