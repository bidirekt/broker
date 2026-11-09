package breakreport

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

type Report struct {
	Stored               bool
	Breaks               int
	CounterpartsBreaking int
	Places               int
	Counterparts         []Counterpart
}

type Counterpart struct {
	Name       string
	Role       model.CounterpartRole
	Version    string
	Registered bool
	Deployable bool
	Breaks     int
	Protocols  map[string]any
}

type Fix struct {
	Participant string
	Action      string
	Checked     bool
}

type Reader struct {
	sections []ProtocolSection
}

func NewReader(sections []ProtocolSection) *Reader {
	return &Reader{sections: sections}
}

func (this *Reader) FromStoredCheck(check model.CompatibilityCheck, results []model.CompatibilityCheckResult) Report {
	counterparts := make([]Counterpart, 0, len(results))
	for _, result := range results {
		counterpart := Counterpart{
			Name:       result.CounterpartName,
			Version:    result.CounterpartVersion.String,
			Registered: result.CounterpartParticipantID.Valid,
			Deployable: result.Deployable,
		}
		if check.Report != nil {
			counterpart.Role = result.Role
		}
		counterparts = append(counterparts, counterpart)
	}
	slices.SortFunc(counterparts, func(a, b Counterpart) int {
		return strings.Compare(a.Name, b.Name)
	})

	if check.Report == nil {
		return Report{Counterparts: counterparts}
	}

	var stored map[string]map[string]json.RawMessage
	if err := json.Unmarshal(check.Report, &stored); err != nil {
		panic(fmt.Errorf("error decoding stored check report: %w", err))
	}

	report := Report{Stored: true, Counterparts: make([]Counterpart, 0, len(counterparts))}
	for _, counterpart := range counterparts {
		checked := CheckedCounterpart{Participant: check.ParticipantName, Environment: check.EnvironmentName, Counterpart: counterpart}
		counterpart.Protocols = make(map[string]any, len(this.sections))
		for _, section := range this.sections {
			reported, err := section.ToSection(checked, stored[counterpart.Name][section.ReportKey()])
			if err != nil {
				panic(fmt.Errorf("error decoding the %s section of a stored check report: %w", section.Protocol(), err))
			}

			counterpart.Breaks += reported.Breaks
			report.Places += reported.Places
			counterpart.Protocols[string(section.Protocol())] = reported.Display
		}

		report.Breaks += counterpart.Breaks
		if counterpart.Breaks > 0 {
			report.CounterpartsBreaking++
		}

		report.Counterparts = append(report.Counterparts, counterpart)
	}

	return report
}
