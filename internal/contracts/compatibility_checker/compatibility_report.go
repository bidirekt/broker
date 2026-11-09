package compatibility_checker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

type Counterpart struct {
	ParticipantID      int64
	ContractID         int64
	ParticipantName    string
	ParticipantVersion null.String
	Role               model.CounterpartRole
}

type CounterpartResult struct {
	Deployable  bool
	Counterpart Counterpart
	Breaks      []ContractBreakingChange
	// CachedBreaks come from a stored verdict: they carry their own tree keys instead of a
	// checked resource, and the pair they belong to is already persisted.
	CachedBreaks  []model.VerdictBreak
	VerdictCached bool
}

func NewCounterpartResult() *CounterpartResult {
	return &CounterpartResult{
		Breaks: make([]ContractBreakingChange, 0),
	}
}

func (this *CounterpartResult) AppendContractBreakChange(b ContractBreakingChange) {
	this.Breaks = append(this.Breaks, b)
	this.Deployable = false
}

func (this *CounterpartResult) AppendCachedBreakChange(b model.VerdictBreak) {
	this.CachedBreaks = append(this.CachedBreaks, b)
	this.Deployable = false
}

type Hierarchical struct {
	Deployable bool
	Version    null.String
	Trees      map[model.Protocol]ReportTree
}

type ContractCompatibilityReport struct {
	ParticipantName string
	Version         string
	Environment     string
	Results         map[string]CounterpartResult
	Hierarchical    map[string]Hierarchical
	protocols       []ProtocolHooks
}

func NewContractCompatibilityReport(
	participantName, version, environment string,
	protocols []ProtocolHooks,
) *ContractCompatibilityReport {
	return &ContractCompatibilityReport{
		ParticipantName: participantName,
		Version:         version,
		Environment:     environment,
		Results:         make(map[string]CounterpartResult),
		Hierarchical:    make(map[string]Hierarchical),
		protocols:       protocols,
	}
}

func (this *ContractCompatibilityReport) AppendResult(counterpartName string, result *CounterpartResult) {
	hierarchical, exists := this.Hierarchical[counterpartName]
	if !exists {
		hierarchical = Hierarchical{
			Deployable: true,
			Trees:      make(map[model.Protocol]ReportTree, len(this.protocols)),
		}

		for _, hooks := range this.protocols {
			hierarchical.Trees[hooks.Protocol()] = hooks.NewReportTree()
		}
	}

	if !hierarchical.Version.Valid {
		hierarchical.Version = result.Counterpart.ParticipantVersion
	}

	for _, breakChange := range result.Breaks {
		role := model.RoleProvider
		if breakChange.CheckedResource.IsConsumer() {
			role = model.RoleConsumer
		}

		hierarchical.appendBreak(
			breakChange.CheckedResource.Protocol,
			breakChange.CheckedResource.Identity.Location(),
			model.ReportBreak{Reason: breakChange.Reason, Role: role, Details: breakChange.Details},
		)
	}

	for _, cachedBreak := range result.CachedBreaks {
		role := model.RoleProvider
		if cachedBreak.Details["consumerName"] == this.ParticipantName {
			role = model.RoleConsumer
		}

		hierarchical.appendBreak(
			cachedBreak.Protocol,
			cachedBreak.Location,
			model.ReportBreak{Reason: cachedBreak.Reason, Role: role, Details: cachedBreak.Details},
		)
	}

	this.Hierarchical[counterpartName] = hierarchical

	existing := this.Results[counterpartName]

	if existing.Breaks == nil {
		existing.Breaks = []ContractBreakingChange{}
	}

	existing.Breaks = append(existing.Breaks, result.Breaks...)
	existing.CachedBreaks = append(existing.CachedBreaks, result.CachedBreaks...)
	existing.VerdictCached = existing.VerdictCached || result.VerdictCached

	// Merged field by field: items of the same counterpart arrive in map order, and the ones
	// that carry no contract must not stop a later one from filling it in.
	if existing.Counterpart.ParticipantID == 0 {
		existing.Counterpart.ParticipantID = result.Counterpart.ParticipantID
	}

	if existing.Counterpart.ContractID == 0 {
		existing.Counterpart.ContractID = result.Counterpart.ContractID
	}

	if !existing.Counterpart.ParticipantVersion.Valid {
		existing.Counterpart.ParticipantVersion = result.Counterpart.ParticipantVersion
	}

	existing.Counterpart.Role = existing.Counterpart.Role.MergedWith(result.Counterpart.Role)

	existing.Counterpart.ParticipantName = counterpartName

	existing.Deployable = len(existing.Breaks)+len(existing.CachedBreaks) == 0

	this.Results[counterpartName] = existing
}

func ToVerdictBreaks(result CounterpartResult) []model.VerdictBreak {
	verdictBreaks := make([]model.VerdictBreak, 0, len(result.Breaks))

	for _, breakChange := range result.Breaks {
		if !breakChange.IsPropertyBreak() {
			continue
		}

		verdictBreaks = append(verdictBreaks, model.VerdictBreak{
			Protocol: breakChange.CheckedResource.Protocol,
			Location: breakChange.CheckedResource.Identity.Location(),
			Reason:   breakChange.Reason,
			Details:  breakChange.Details,
		})
	}

	return verdictBreaks
}

func (this *ContractCompatibilityReport) Deployable() bool {
	for _, result := range this.Results {
		if !result.Deployable {
			return false
		}
	}

	return true
}

func (this *Hierarchical) appendBreak(protocol model.Protocol, location map[string]string, reportBreak model.ReportBreak) {
	tree, registered := this.Trees[protocol]
	if !registered {
		panic(fmt.Errorf("protocol %q has no report tree", protocol))
	}

	tree.Append(location, reportBreak)

	this.Deployable = false
}

type hierarchicalField struct {
	key   string
	value any
}

// MarshalJSON writes the fields in a fixed order, the trees last and sorted by their key, so
// the bytes of a stored report never depend on map order.
func (this Hierarchical) MarshalJSON() ([]byte, error) {
	fields := []hierarchicalField{
		{"deployable", this.Deployable},
		{"participantVersion", this.Version},
	}

	trees := slices.SortedFunc(maps.Values(this.Trees), func(a, b ReportTree) int {
		return strings.Compare(a.ReportKey(), b.ReportKey())
	})
	for _, tree := range trees {
		fields = append(fields, hierarchicalField{tree.ReportKey(), tree})
	}

	var buffer bytes.Buffer

	buffer.WriteByte('{')

	for i, field := range fields {
		if i > 0 {
			buffer.WriteByte(',')
		}

		key, err := json.Marshal(field.key)
		if err != nil {
			return nil, err
		}

		value, err := json.Marshal(field.value)
		if err != nil {
			return nil, err
		}

		buffer.Write(key)
		buffer.WriteByte(':')
		buffer.Write(value)
	}

	buffer.WriteByte('}')

	return buffer.Bytes(), nil
}
