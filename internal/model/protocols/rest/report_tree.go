package rest

import (
	"cmp"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

// ReportTree indexes breaks by endpoint, method and interaction.
type ReportTree map[string]map[string]map[string][]model.ReportBreak

func (this ReportTree) ReportKey() string {
	return ReportKey
}

func (this ReportTree) Append(location map[string]string, reportBreak model.ReportBreak) {
	endpoint, method, interaction := location[EndpointKey], location[MethodKey], location[InteractionKey]

	if _, ok := this[endpoint]; !ok {
		this[endpoint] = make(map[string]map[string][]model.ReportBreak)
	}

	if _, ok := this[endpoint][method]; !ok {
		this[endpoint][method] = make(map[string][]model.ReportBreak)
	}

	breaks := append(this[endpoint][method][interaction], reportBreak)
	slices.SortStableFunc(breaks, func(a, b model.ReportBreak) int {
		return cmp.Or(
			strings.Compare(a.Details["property"], b.Details["property"]),
			strings.Compare(string(a.Reason), string(b.Reason)),
		)
	})
	this[endpoint][method][interaction] = breaks
}
