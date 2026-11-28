package compatibility_checker

import (
	"context"

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
	// Names a field of the resource whose schema name means less to a reader than the resource itself.
	FieldName(resource *model.PersistedResource, site schemagraph.Site) (name string, named bool)
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

// ExtraCounterparts is the port of a protocol whose counterparts do not come from the deployments directly, or
// whose breaks a comparison with one contract doesn't find. The checker leaves the providers it takes over, for
// the resources of its protocol, to the pairs it hands back.
type ExtraCounterparts interface {
	ExtraCounterparts(ctx context.Context, contract *model.PersistedContract, environment *model.Environment) ExtraPairs
}

type ExtraPairs struct {
	Providers map[string]bool
	Pairs     []ExtraPair
}

// ExtraPair compares the resources a consumer reads with a provider held in memory; when the provider has no
// contract, its ready break applies to each of them instead. When the provider is the side under check, Base is
// the provider before the change: a break it also has is not the change's.
type ExtraPair struct {
	Resources   []model.PersistedResource
	Contract    *model.PersistedContract
	Counterpart Counterpart
	ReadyBreak  ReadyBreak
	Base        *model.PersistedContract
	// Details are added to the details of every break the pair finds.
	Details map[string]string
}

type ReadyBreak struct {
	Reason  model.BreakingReason
	Details map[string]string
}
