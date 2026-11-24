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
	reading := this.hooksOf(consumer.Protocol).BodyReading(consumer.Body)

	consumerBody := schemagraph.New(consumer.Schemas).Root(consumer.Bodies, reading.Join)
	providerBody := schemagraph.New(provider.Schemas).Root(provider.Bodies, reading.Join)

	reader, producer := providerBody, consumerBody
	if reading.Rules.ReaderIsConsumer {
		reader, producer = consumerBody, providerBody
	}

	return compareBodies(checked, counterpart, reading.Rules, reader, producer)
}

func consumerAndProvider(checked, counterpart *model.PersistedResource) (consumer, provider *model.PersistedResource) {
	if checked.IsConsumer() {
		return checked, counterpart
	}

	return counterpart, checked
}

func acceptsType(reader, producer string) bool {
	if reader == producer {
		return true
	}

	return (reader == model.NumberType && (producer == model.IntegerType || producer == model.FloatType)) ||
		(reader == model.StringType && producer == model.EnumType)
}
