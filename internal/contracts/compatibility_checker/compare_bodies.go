package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type readingRules struct {
	readerIsConsumer bool
	missing          BreakingReason
	optional         BreakingReason
	nullable         BreakingReason
}

// Optional and nullable belong to the fields a node is reached through, so each pair of them is compared once;
// the type and the children belong to the shapes the refs lead to, so each pair of shapes is compared once.
// A schema used in many places breaks once, and recursion ends.
type bodyComparison struct {
	checked        *model.PersistedResource
	counterpart    *model.PersistedResource
	rules          readingRules
	comparedSites  map[[2]string]bool
	comparedShapes map[[2]string]bool
	breaks         []ContractBreakingChange
}

func compareBodies(
	checked *model.PersistedResource,
	counterpart *model.PersistedResource,
	rules readingRules,
	reader, producer schemagraph.Node,
) []ContractBreakingChange {
	comparison := &bodyComparison{
		checked:        checked,
		counterpart:    counterpart,
		rules:          rules,
		comparedSites:  map[[2]string]bool{},
		comparedShapes: map[[2]string]bool{},
	}

	comparison.compareRead(reader, producer, "$")

	return comparison.breaks
}

func (this *bodyComparison) compareRead(reader, producer schemagraph.Node, path string) {
	if !reader.Exists() {
		return
	}

	if !producer.Exists() {
		if !reader.Optional() {
			this.breakAt(this.rules.missing, path, reader, producer)
		}

		return
	}

	sites := [2]string{reader.SiteKey(), producer.SiteKey()}
	if this.comparedSites[sites] {
		return
	}

	this.comparedSites[sites] = true

	shapes := [2]string{reader.ShapeKey(), producer.ShapeKey()}

	if !acceptsType(reader.Type(), producer.Type()) {
		if !this.comparedShapes[shapes] {
			this.comparedShapes[shapes] = true
			this.breakAt(ReasonPropertyTypeMismatch, path, reader, producer)
		}

		return
	}

	if !reader.Optional() && producer.Optional() {
		this.breakAt(this.rules.optional, path, reader, producer)
	}

	if producer.Nullable() && !reader.Nullable() {
		this.breakAt(this.rules.nullable, path, reader, producer)
	}

	if this.comparedShapes[shapes] {
		return
	}

	this.comparedShapes[shapes] = true

	for _, segment := range reader.Segments() {
		this.compareRead(reader.Child(segment), producer.Child(segment), model.ChildPath(path, segment))
	}
}

func (this *bodyComparison) breakAt(reason BreakingReason, path string, reader, producer schemagraph.Node) {
	consumer, provider := producer, reader
	if this.rules.readerIsConsumer {
		consumer, provider = reader, producer
	}

	this.breaks = append(this.breaks, NewPropertyBreakChange(this.checked, this.counterpart, reason, path, consumer, provider))
}
