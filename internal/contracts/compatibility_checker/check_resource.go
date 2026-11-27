package compatibility_checker

import (
	"slices"

	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

// A body of FieldArguments is the arguments of the resource's field: its breaks name the argument, as a nested field's do.
type BodyReading struct {
	Rules          ReadingRules
	Join           schemagraph.Join
	FieldArguments bool
}

func (this *CompatibilityChecker) checkResources(checked *model.PersistedResource, counterpart *model.PersistedResource) []ContractBreakingChange {
	consumer, provider := consumerAndProvider(checked, counterpart)
	hooks := this.hooksOf(consumer.Protocol)
	reading := hooks.BodyReading(consumer.Body)

	body := compareBodies(checked, counterpart, reading.Rules,
		schemaGraph(hooks, consumer.Schemas).Root(consumer.Bodies, reading.Join),
		schemaGraph(hooks, provider.Schemas).Root(provider.Bodies, reading.Join),
	)

	breaks := body.breaks
	if reading.FieldArguments {
		for index := range breaks {
			breaks[index].placeInArgumentsOf("$")
		}
	}

	arguments := hooks.BodyReading(model.Input)

	for _, field := range body.argumentFields {
		comparison := compareBodies(checked, counterpart, arguments.Rules, field.consumer.ArgumentsRoot(arguments.Join), field.provider.ArgumentsRoot(arguments.Join))
		for _, argumentBreak := range comparison.breaks {
			argumentBreak.placeInArgumentsOf(field.path)
			breaks = append(breaks, argumentBreak)
		}
	}

	return breaks
}

func schemaGraph(hooks ProtocolHooks, schemas map[string]model.Schemas) *schemagraph.Graph {
	graph := schemagraph.New(schemas)
	if namer, names := hooks.(FieldNamer); names {
		graph.NamingFields(namer.FieldName)
	}

	if namer, names := hooks.(TypeNamer); names {
		graph.NamingTypes(namer.TypeName)
	}

	return graph
}

func consumerAndProvider(checked, counterpart *model.PersistedResource) (consumer, provider *model.PersistedResource) {
	if checked.IsConsumer() {
		return checked, counterpart
	}

	return counterpart, checked
}

func acceptsType(reader, producer schemagraph.Node, rules ReadingRules) bool {
	if reader.Type() == producer.Type() {
		return reader.Type() != model.ScalarType || reader.Scalar() == producer.Scalar()
	}

	return (reader.Type() == model.NumberType && (producer.Type() == model.IntegerType || producer.Type() == model.FloatType)) ||
		(reader.Type() == model.StringType && producer.Type() == model.EnumType) ||
		(reader.Type() == model.ObjectType && producer.Type() == model.UnionType && (declaresFields(producer) || readsOnly(reader, rules.ImplicitFields))) ||
		slices.Contains(rules.Widenings, Widening{Reader: schemaTypeOf(reader), Producer: schemaTypeOf(producer)})
}

// A union may declare the fields every member has; read as an object, it is read by them.
func declaresFields(union schemagraph.Node) bool {
	return slices.ContainsFunc(union.Segments(), func(segment string) bool { return !model.IsMemberSegment(segment) })
}

func readsOnly(reader schemagraph.Node, fields []string) bool {
	segments := reader.Segments()

	return len(segments) > 0 && !slices.ContainsFunc(segments, func(segment string) bool { return !slices.Contains(fields, segment) })
}

func schemaTypeOf(node schemagraph.Node) SchemaType {
	return SchemaType{Type: node.Type(), Scalar: node.Scalar()}
}
