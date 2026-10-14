package compatibility_checker

import (
	"slices"

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
	this.compareReadRequiring(reader, producer, path, !reader.Optional())
}

func (this *bodyComparison) compareReadRequiring(reader, producer schemagraph.Node, path string, required bool) {
	if !reader.Exists() {
		return
	}

	if !producer.Exists() {
		if required {
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

	if required && producer.Optional() {
		this.breakAt(this.rules.optional, path, reader, producer)
	}

	if producer.Nullable() && !reader.Nullable() {
		this.breakAt(this.rules.nullable, path, reader, producer)
	}

	if this.comparedShapes[shapes] {
		return
	}

	this.comparedShapes[shapes] = true

	readSegments := reader.Segments()

	for _, segment := range readSegments {
		if segment == model.ValuesSegment {
			this.compareValuesRead(reader.Child(segment), producer, path, readSegments)

			continue
		}

		this.compareRead(reader.Child(segment), producer.Child(segment), model.ChildPath(path, segment))
	}
}

// A map requires none of its keys, so each fixed key the reader does not read on its own is compared against the
// values without the optional rule.
func (this *bodyComparison) compareValuesRead(values, producer schemagraph.Node, path string, readSegments []string) {
	producedValues := producer.Child(model.ValuesSegment)
	producedKeys := fixedKeysOf(producer)

	if producedValues.Exists() || len(producedKeys) == 0 {
		this.compareReadRequiring(values, producedValues, model.ChildPath(path, model.ValuesSegment), len(producedKeys) == 0 && !values.Optional())
	}

	for _, key := range producedKeys {
		if !slices.Contains(readSegments, key) {
			this.compareReadRequiring(values, producer.Child(key), model.ChildPath(path, key), false)
		}
	}
}

func fixedKeysOf(node schemagraph.Node) []string {
	return slices.DeleteFunc(node.Segments(), func(segment string) bool {
		return segment == model.ItemsSegment || segment == model.ValuesSegment
	})
}

func (this *bodyComparison) breakAt(reason BreakingReason, path string, reader, producer schemagraph.Node) {
	consumer, provider := producer, reader
	if this.rules.readerIsConsumer {
		consumer, provider = reader, producer
	}

	this.breaks = append(this.breaks, NewPropertyBreakChange(this.checked, this.counterpart, reason, path, consumer, provider))
}
