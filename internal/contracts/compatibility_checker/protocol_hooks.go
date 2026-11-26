package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type ProtocolHooks interface {
	Protocol() model.Protocol
	NewReportTree() ReportTree
	BodyReading(body model.BodyRole) BodyReading
}

type NotFoundRefiner interface {
	// Called only when the counterparts hold a provider hint for the consumer's provider hash.
	RefineNotFound(hint any) (reason model.BreakingReason, details map[string]string, refined bool)
}

type RemovedRefiner interface {
	// Called once per removed provider resource, before its consumers are walked.
	RefineRemoved(removed *model.PersistedResource, checkedResources map[string]model.PersistedResource) (reason model.BreakingReason, details map[string]string, refined bool)
}

type FieldNamer interface {
	// Names a field of a schema the protocol synthesizes, whose own name means nothing to a reader.
	FieldName(site schemagraph.Site) (name string, named bool)
}

type TypeNamer interface {
	// Names the type of a node as the protocol writes it.
	TypeName(node schemagraph.Node) (name string, named bool)
}

type ReportTree interface {
	ReportKey() string
	Append(location map[string]string, reportBreak model.ReportBreak)
	IsEmpty() bool
}
