package compatibility_checker

import (
	"cmp"
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

type HierarchicalInteraction map[string][]ContractBreakingChange

type HierarchicalMethod map[string]HierarchicalInteraction

type HierarchicalEndpoint map[string]HierarchicalMethod

type Hierarchical struct {
	Deployable bool                 `json:"deployable"`
	Version    null.String          `json:"participantVersion"`
	Endpoints  HierarchicalEndpoint `json:"endpoints"`
}

type ContractCompatibilityReport struct {
	ParticipantName string
	Version         string
	Environment     string
	Results         map[string]CounterpartResult
	Hierarchical    map[string]Hierarchical
}

func NewContractCompatibilityReport(
	participantName, version, environment string,
) *ContractCompatibilityReport {
	return &ContractCompatibilityReport{
		ParticipantName: participantName,
		Version:         version,
		Environment:     environment,
		Results:         make(map[string]CounterpartResult),
		Hierarchical:    make(map[string]Hierarchical),
	}
}

func (this *ContractCompatibilityReport) AppendResult(counterpartName string, result *CounterpartResult) {

	hierarchical, exists := this.Hierarchical[counterpartName]
	if !exists {
		hierarchical = Hierarchical{
			Deployable: true,
			Endpoints:  make(HierarchicalEndpoint),
		}
	}

	if !hierarchical.Version.Valid {
		hierarchical.Version = result.Counterpart.ParticipantVersion
	}

	for _, breakChange := range result.Breaks {
		breakChange.Role = model.RoleProvider
		if breakChange.CheckedResource.IsConsumer() {
			breakChange.Role = model.RoleConsumer
		}

		hierarchical.appendBreak(
			breakChange.CheckedResource.Endpoint,
			breakChange.CheckedResource.Method,
			breakChange.InteractionKey(),
			breakChange,
		)
	}

	for _, cachedBreak := range result.CachedBreaks {
		role := model.RoleProvider
		if cachedBreak.Details["consumerName"] == this.ParticipantName {
			role = model.RoleConsumer
		}

		hierarchical.appendBreak(
			cachedBreak.Endpoint,
			cachedBreak.Method,
			cachedBreak.Interaction,
			ContractBreakingChange{
				Reason:  cachedBreak.Reason,
				Role:    role,
				Details: cachedBreak.Details,
			},
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

func (this *ContractCompatibilityReport) Deployable() bool {
	for _, result := range this.Results {
		if !result.Deployable {
			return false
		}
	}

	return true
}

func (this *Hierarchical) appendBreak(
	endpoint, method, interaction string,
	breakChange ContractBreakingChange,
) {
	if _, ok := this.Endpoints[endpoint]; !ok {
		this.Endpoints[endpoint] = make(HierarchicalMethod)
	}

	if _, ok := this.Endpoints[endpoint][method]; !ok {
		this.Endpoints[endpoint][method] = make(HierarchicalInteraction)
	}

	breaks := append(this.Endpoints[endpoint][method][interaction], breakChange)
	slices.SortStableFunc(breaks, func(a, b ContractBreakingChange) int {
		return cmp.Or(
			strings.Compare(a.Details["property"], b.Details["property"]),
			strings.Compare(string(a.Reason), string(b.Reason)),
		)
	})
	this.Endpoints[endpoint][method][interaction] = breaks

	this.Deployable = false
}
