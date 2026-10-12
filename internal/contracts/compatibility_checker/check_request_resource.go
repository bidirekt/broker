package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

var requestRules = readingRules{
	readerIsConsumer: false,
	missing:          ReasonPropertyMissingInConsumer,
	optional:         ReasonPropertyOptionalInConsumerRequiredInProvider,
	nullable:         ReasonPropertyNullableInConsumerNotNullableInProvider,
}

func checkRequestResource(
	checked *model.PersistedResource,
	counterpart *model.PersistedResource,
	consumer *model.PersistedResource,
	provider *model.PersistedResource,
) []ContractBreakingChange {
	sent := schemagraph.New(consumer.Schemas).Root(consumer.Bodies, schemagraph.Senders)
	read := schemagraph.New(provider.Schemas).Root(provider.Bodies, schemagraph.Senders)

	return compareBodies(checked, counterpart, requestRules, read, sent)
}
