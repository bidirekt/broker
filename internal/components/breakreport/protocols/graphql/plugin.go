package graphql

import (
	"embed"
	"encoding/json"
	"io/fs"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

//go:embed graphql_operations.html operations.css
var sectionFiles embed.FS

// Plugin draws no break yet: no GraphQL break is reported, so its section is always empty.
type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (this *Plugin) Protocol() model.Protocol {
	return graphqlmodel.Protocol
}

func (this *Plugin) ReportKey() string {
	return graphqlmodel.ReportKey
}

func (this *Plugin) Files() fs.FS {
	return sectionFiles
}

func (this *Plugin) Template() string {
	return "graphql_operations"
}

func (this *Plugin) ToSection(breakreport.CheckedCounterpart, json.RawMessage) (breakreport.Section, error) {
	return breakreport.Section{}, nil
}
