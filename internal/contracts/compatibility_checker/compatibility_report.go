package compatibility_checker

import (
	"cmp"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

type IncompatibleCounterpart struct {
	ParticipantID      int64       `json:"-"`
	ContractID         int64       `json:"-"`
	ParticipantName    string      `json:"participantName"`
	ParticipantVersion null.String `json:"participantVersion"`
}

// InteractionKey is the wire spelling of the checked resource's interaction: "request"
// for requests, the raw status code for responses.
func (this *ContractBreakingChange) InteractionKey() string {
	switch this.CheckedResource.Interaction {
	case model.RestRequest:
		return "request"
	case model.RestResponse:
		return this.CheckedResource.ResponseStatusCode.String
	default:
		return this.CheckedResource.Interaction.String()
	}
}

// IsPropertyBreak reports whether the break comes from a property diff, as opposed to the
// environment checks that depend on deployments and never become a stored verdict.
func (this *ContractBreakingChange) IsPropertyBreak() bool {
	return this.Reason != ReasonProviderResourceNotFound &&
		this.Reason != ReasonProviderResourceNotDeployedInEnvironment &&
		this.Reason != ReasonProviderResourceRemovedButStillConsumed
}

type IncompatibleItem struct {
	Deployable              bool                     `json:"deployable"`
	IncompatibleCounterpart IncompatibleCounterpart  `json:"incompatibleCounterpart"`
	Breaks                  []ContractBreakingChange `json:"breaks"`
	// CachedBreaks come from a stored verdict: they carry their own tree keys instead of a
	// checked resource, and the pair they belong to is already persisted.
	CachedBreaks  []model.VerdictBreak `json:"-"`
	VerdictCached bool                 `json:"-"`
}

func NewIncompatibleItem() *IncompatibleItem {
	return &IncompatibleItem{
		Breaks: make([]ContractBreakingChange, 0),
	}
}

func (this *IncompatibleItem) AppendContractBreakChange(b ContractBreakingChange) {
	this.Breaks = append(this.Breaks, b)
	this.Deployable = false
}

func (this *IncompatibleItem) AppendCachedBreakChange(b model.VerdictBreak) {
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
	Results         map[string]IncompatibleItem
	Hierarchical    map[string]Hierarchical
}

func NewContractCompatibilityReport(
	participantName, version, environment string,
) *ContractCompatibilityReport {
	return &ContractCompatibilityReport{
		ParticipantName: participantName,
		Version:         version,
		Environment:     environment,
		Results:         make(map[string]IncompatibleItem),
		Hierarchical:    make(map[string]Hierarchical),
	}
}

func (this *ContractCompatibilityReport) AppendResult(dependency string, result *IncompatibleItem) {

	hierarchical, exists := this.Hierarchical[dependency]
	if !exists {
		hierarchical = Hierarchical{
			Deployable: true,
			Endpoints:  make(HierarchicalEndpoint),
		}
	}

	if !hierarchical.Version.Valid {
		hierarchical.Version = result.IncompatibleCounterpart.ParticipantVersion
	}

	for _, breakChange := range result.Breaks {
		breakChange.Role = RoleProvider
		if breakChange.CheckedResource.IsConsumer() {
			breakChange.Role = RoleConsumer
		}

		hierarchical.appendBreak(
			breakChange.CheckedResource.Endpoint,
			breakChange.CheckedResource.Method,
			breakChange.InteractionKey(),
			breakChange,
		)
	}

	for _, cachedBreak := range result.CachedBreaks {
		role := RoleProvider
		if cachedBreak.Details["consumerName"] == this.ParticipantName {
			role = RoleConsumer
		}

		hierarchical.appendBreak(
			cachedBreak.Endpoint,
			cachedBreak.Method,
			cachedBreak.Interaction,
			ContractBreakingChange{
				Reason:  BreakingReason(cachedBreak.Reason),
				Role:    role,
				Details: cachedBreak.Details,
			},
		)
	}

	this.Hierarchical[dependency] = hierarchical

	existing := this.Results[dependency]

	if existing.Breaks == nil {
		existing.Breaks = []ContractBreakingChange{}
	}

	existing.Breaks = append(existing.Breaks, result.Breaks...)
	existing.CachedBreaks = append(existing.CachedBreaks, result.CachedBreaks...)
	existing.VerdictCached = existing.VerdictCached || result.VerdictCached

	// Merged field by field: items of the same counterpart arrive in map order, and the ones
	// that carry no contract must not stop a later one from filling it in.
	if existing.IncompatibleCounterpart.ParticipantID == 0 {
		existing.IncompatibleCounterpart.ParticipantID = result.IncompatibleCounterpart.ParticipantID
	}

	if existing.IncompatibleCounterpart.ContractID == 0 {
		existing.IncompatibleCounterpart.ContractID = result.IncompatibleCounterpart.ContractID
	}

	if !existing.IncompatibleCounterpart.ParticipantVersion.Valid {
		existing.IncompatibleCounterpart.ParticipantVersion = result.IncompatibleCounterpart.ParticipantVersion
	}

	existing.IncompatibleCounterpart.ParticipantName = dependency

	existing.Deployable = len(existing.Breaks)+len(existing.CachedBreaks) == 0

	this.Results[dependency] = existing
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
