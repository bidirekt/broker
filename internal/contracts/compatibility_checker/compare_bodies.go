package compatibility_checker

import (
	"slices"

	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type readingRules struct {
	readerIsConsumer bool
	missing          model.BreakingReason
	optional         model.BreakingReason
	nullable         model.BreakingReason
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
			this.breakAt(model.ReasonPropertyTypeMismatch, path, reader, producer)
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

	if reader.Type() == model.UnionType {
		this.compareUnionMembers(reader, producer, path)

		return
	}

	this.compareEnumValues(reader, producer, path)

	for _, segment := range reader.Segments() {
		this.compareRead(reader.Child(segment), producer.Child(segment), model.ChildPath(path, segment))
	}
}

// The provider owns how a property is provided, so whichever side reads, every consumer value must be among the provider's.
func (this *bodyComparison) compareEnumValues(reader, producer schemagraph.Node, path string) {
	if reader.Type() != model.EnumType || producer.Type() != model.EnumType {
		return
	}

	consumer, provider := this.consumerAndProvider(reader, producer)
	for _, member := range consumer.Members() {
		if !slices.Contains(provider.Members(), member) {
			this.breaks = append(this.breaks, NewMemberBreakChange(this.checked, this.counterpart, member, path, consumer, provider))
		}
	}
}

// Like enum values, every consumer member must be among the provider's; the members both declare are read like any schema.
func (this *bodyComparison) compareUnionMembers(reader, producer schemagraph.Node, path string) {
	consumer, provider := this.consumerAndProvider(reader, producer)

	for _, segment := range consumer.Segments() {
		if !provider.Child(segment).Exists() {
			this.breaks = append(this.breaks, NewMemberBreakChange(this.checked, this.counterpart, model.MemberKey(segment), path, consumer, provider))

			continue
		}

		this.compareRead(reader.Child(segment), producer.Child(segment), model.ChildPath(path, segment))
	}
}

func (this *bodyComparison) breakAt(reason model.BreakingReason, path string, reader, producer schemagraph.Node) {
	consumer, provider := this.consumerAndProvider(reader, producer)

	this.breaks = append(this.breaks, NewPropertyBreakChange(this.checked, this.counterpart, reason, path, consumer, provider))
}

func (this *bodyComparison) consumerAndProvider(reader, producer schemagraph.Node) (consumer, provider schemagraph.Node) {
	if this.rules.readerIsConsumer {
		return reader, producer
	}

	return producer, reader
}
