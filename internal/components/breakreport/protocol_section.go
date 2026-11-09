package breakreport

import (
	"encoding/json"

	"github.com/bidirekt/broker/internal/model"
)

type ProtocolSection interface {
	Protocol() model.Protocol
	ReportKey() string
	// tree is nil when the stored counterpart has no tree under ReportKey.
	ToSection(checked CheckedCounterpart, tree json.RawMessage) (Section, error)
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
