package breakreport

import (
	"encoding/json"
	"io/fs"

	"github.com/bidirekt/broker/internal/model"
)

type ProtocolSection interface {
	Protocol() model.Protocol
	ReportKey() string
	// tree is nil when the stored counterpart has no tree under ReportKey.
	ToSection(checked CheckedCounterpart, tree json.RawMessage) (Section, error)
	// Files holds the section's stylesheet, <ReportKey>.css, and its *.html templates, parsed with the pages' layouts and partials.
	Files() fs.FS
	Template() string
}

type CheckedCounterpart struct {
	Participant string
	Environment string
	Counterpart Counterpart
}

type Section struct {
	Display any
	Breaks  int
	Places  int
}
