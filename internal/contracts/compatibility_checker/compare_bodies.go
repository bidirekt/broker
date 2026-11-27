package compatibility_checker

import (
	"slices"

	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

// Unknown is the break of a field only the producer declares; left empty, the reader ignores the field.
type ReadingRules struct {
	ReaderIsConsumer  bool
	Missing           model.BreakingReason
	Optional          model.BreakingReason
	Nullable          model.BreakingReason
	Unknown           model.BreakingReason
	OptionalMustExist bool
	SingleToList      bool
	Widenings         []Widening
	ImplicitFields    []string
}

type SchemaType struct{ Type, Scalar string }

// A Widening lets a reader take a producer type beyond the ones every protocol accepts.
type Widening struct{ Reader, Producer SchemaType }

// Optional and nullable belong to the fields a node is reached through, so each pair of them is compared once;
// the type and the children belong to the shapes the refs lead to, so each pair of shapes is compared once.
// A schema used in many places breaks once, and recursion ends.
type bodyComparison struct {
	checked        *model.PersistedResource
	counterpart    *model.PersistedResource
	rules          ReadingRules
	comparedSites  map[[2]string]bool
	comparedShapes map[[2]string]bool
	breaks         []ContractBreakingChange
	argumentFields []argumentField
}

type argumentField struct {
	path     string
	consumer schemagraph.Node
	provider schemagraph.Node
}

func compareBodies(
	checked *model.PersistedResource,
	counterpart *model.PersistedResource,
	rules ReadingRules,
	consumerBody, providerBody schemagraph.Node,
) *bodyComparison {
	comparison := &bodyComparison{
		checked:        checked,
		counterpart:    counterpart,
		rules:          rules,
		comparedSites:  map[[2]string]bool{},
		comparedShapes: map[[2]string]bool{},
	}

	reader, producer := providerBody, consumerBody
	if rules.ReaderIsConsumer {
		reader, producer = consumerBody, providerBody
	}

	comparison.compareRead(reader, producer, "$")

	return comparison
}

func (this *bodyComparison) compareRead(reader, producer schemagraph.Node, path string) {
	if !reader.Exists() {
		return
	}

	if !producer.Exists() {
		if !reader.Optional() || this.rules.OptionalMustExist {
			this.breakAt(this.rules.Missing, path, reader, producer)
		}

		return
	}

	sites := [2]string{reader.SiteKey(), producer.SiteKey()}
	if this.comparedSites[sites] {
		return
	}

	this.comparedSites[sites] = true

	if reader.Arguments() != "" || producer.Arguments() != "" {
		consumer, provider := this.consumerAndProvider(reader, producer)
		this.argumentFields = append(this.argumentFields, argumentField{path: path, consumer: consumer, provider: provider})
	}

	shape := reader
	for this.rules.SingleToList && shape.Type() == model.ArrayType && producer.Type() != model.ArrayType {
		shape = shape.Child(model.ItemsSegment)
	}

	shapes := [2]string{shape.ShapeKey(), producer.ShapeKey()}

	if !acceptsType(shape, producer, this.rules) {
		if !this.comparedShapes[shapes] {
			this.comparedShapes[shapes] = true
			this.breakAt(model.ReasonPropertyTypeMismatch, path, reader, producer)
		}

		return
	}

	if !reader.Optional() && producer.Optional() {
		this.breakAt(this.rules.Optional, path, reader, producer)
	}

	if producer.Nullable() && !reader.Nullable() {
		this.breakAt(this.rules.Nullable, path, reader, producer)
	}

	if this.comparedShapes[shapes] {
		return
	}

	this.comparedShapes[shapes] = true

	if shape.Type() == model.UnionType {
		this.compareUnionMembers(shape, producer, path)
		this.compareFields(shape, producer, path)

		return
	}

	this.compareEnumValues(shape, producer, path)
	this.compareFields(shape, producer, path)
	this.compareUnknown(shape, producer, path)
}

// The fields read on a union are the ones it declares itself, like an interface's; its members are compared apart.
func (this *bodyComparison) compareFields(reader, producer schemagraph.Node, path string) {
	for _, segment := range reader.Segments() {
		if model.IsMemberSegment(segment) || (slices.Contains(this.rules.ImplicitFields, segment) && !producer.Child(segment).Exists()) {
			continue
		}

		this.compareRead(reader.Child(segment), producer.Child(segment), model.ChildPath(path, segment))
	}
}

func (this *bodyComparison) compareUnknown(reader, producer schemagraph.Node, path string) {
	if this.rules.Unknown == "" {
		return
	}

	for _, segment := range producer.Segments() {
		if !reader.Child(segment).Exists() {
			this.breakAt(this.rules.Unknown, model.ChildPath(path, segment), reader.Child(segment), producer.Child(segment))
		}
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
		if !model.IsMemberSegment(segment) {
			continue
		}

		if !provider.Child(segment).Exists() {
			this.breaks = append(this.breaks, NewMemberBreakChange(this.checked, this.counterpart, model.MemberKey(segment), path, consumer.Declaring(segment), provider))

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
	if this.rules.ReaderIsConsumer {
		return reader, producer
	}

	return producer, reader
}
