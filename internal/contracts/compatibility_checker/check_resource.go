package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type BodyReading struct {
	Rules ReadingRules
	Join  schemagraph.Join
}

func (this *CompatibilityChecker) checkResources(checked *model.PersistedResource, counterpart *model.PersistedResource) []ContractBreakingChange {
	consumer, provider := consumerAndProvider(checked, counterpart)
	hooks := this.hooksOf(consumer.Protocol)
	reading := hooks.BodyReading(consumer.Body)

	body := compareBodies(checked, counterpart, reading.Rules,
		schemagraph.New(consumer.Schemas).Root(consumer.Bodies, reading.Join),
		schemagraph.New(provider.Schemas).Root(provider.Bodies, reading.Join),
	)

	breaks := body.breaks
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

func consumerAndProvider(checked, counterpart *model.PersistedResource) (consumer, provider *model.PersistedResource) {
	if checked.IsConsumer() {
		return checked, counterpart
	}

	return counterpart, checked
}

func acceptsType(reader, producer schemagraph.Node) bool {
	if reader.Type() == producer.Type() {
		return reader.Scalar() == producer.Scalar()
	}

	return (reader.Type() == model.NumberType && (producer.Type() == model.IntegerType || producer.Type() == model.FloatType)) ||
		(reader.Type() == model.StringType && producer.Type() == model.EnumType)
}
