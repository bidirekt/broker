package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type bodyReading struct {
	rules readingRules
	join  schemagraph.Join
}

var inputRules = readingRules{
	readerIsConsumer: false,
	missing:          model.ReasonPropertyMissingInConsumer,
	optional:         model.ReasonPropertyOptionalInConsumerRequiredInProvider,
	nullable:         model.ReasonPropertyNullableInConsumerNotNullableInProvider,
}

var outputRules = readingRules{
	readerIsConsumer: true,
	missing:          model.ReasonPropertyMissingInProvider,
	optional:         model.ReasonPropertyOptionalInProviderRequiredInConsumer,
	nullable:         model.ReasonPropertyNullableInProviderNotNullableInConsumer,
}

var bodyReadings = map[model.BodyRole]bodyReading{
	model.Input:  {rules: inputRules, join: schemagraph.Senders},
	model.Output: {rules: outputRules, join: schemagraph.Readers},
}

func checkResources(checked *model.PersistedResource, counterpart *model.PersistedResource) []ContractBreakingChange {
	consumer, provider := consumerAndProvider(checked, counterpart)
	reading := bodyReadings[consumer.Body]

	consumerBody := schemagraph.New(consumer.Schemas).Root(consumer.Bodies, reading.join)
	providerBody := schemagraph.New(provider.Schemas).Root(provider.Bodies, reading.join)

	reader, producer := providerBody, consumerBody
	if reading.rules.readerIsConsumer {
		reader, producer = consumerBody, providerBody
	}

	return compareBodies(checked, counterpart, reading.rules, reader, producer)
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
