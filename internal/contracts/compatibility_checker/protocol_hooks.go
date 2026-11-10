package compatibility_checker

import "github.com/bidirekt/broker/internal/model"

type ProtocolHooks interface {
	Protocol() model.Protocol
	NewReportTree() ReportTree
	// Called only when the counterparts hold a provider hint for the consumer's provider hash.
	RefineNotFound(hint any) (reason model.BreakingReason, details map[string]string, refined bool)
	// Called once per removed provider resource, before its consumers are walked.
	RefineRemoved(removed *model.PersistedResource, checkedResources map[string]model.PersistedResource) (reason model.BreakingReason, details map[string]string, refined bool)
}

type ReportTree interface {
	ReportKey() string
	Append(location map[string]string, reportBreak model.ReportBreak)
}
