package graphql

import (
	"cmp"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

// ReportTree indexes breaks by operation, field and interaction.
type ReportTree map[string]map[string]map[string][]model.ReportBreak

func (this ReportTree) ReportKey() string {
	return ReportKey
}

func (this ReportTree) IsEmpty() bool {
	return len(this) == 0
}

// Append keeps a resource break once per field, under returns: every field has returns, so
// the break lands in the same place whichever interaction reports it first.
func (this ReportTree) Append(location map[string]string, reportBreak model.ReportBreak) {
	operation, field, interaction := location[OperationKey], location[FieldKey], location[InteractionKey]

	if !reportBreak.Reason.IsPropertyBreak() {
		interaction = ReturnsInteraction
		if slices.ContainsFunc(this[operation][field][interaction], func(reported model.ReportBreak) bool {
			return reported.Reason == reportBreak.Reason
		}) {
			return
		}
	}

	if _, ok := this[operation]; !ok {
		this[operation] = make(map[string]map[string][]model.ReportBreak)
	}

	if _, ok := this[operation][field]; !ok {
		this[operation][field] = make(map[string][]model.ReportBreak)
	}

	breaks := append(this[operation][field][interaction], reportBreak)
	slices.SortStableFunc(breaks, func(a, b model.ReportBreak) int {
		return cmp.Or(
			strings.Compare(a.Details["property"], b.Details["property"]),
			strings.Compare(a.Details["argument"], b.Details["argument"]),
			strings.Compare(string(a.Reason), string(b.Reason)),
		)
	})
	this[operation][field][interaction] = breaks
}
