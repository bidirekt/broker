package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

var responseRules = readingRules{
	readerIsConsumer: true,
	missing:          ReasonPropertyMissingInProvider,
	optional:         ReasonPropertyOptionalInProviderRequiredInConsumer,
	nullable:         ReasonPropertyNullableInProviderNotNullableInConsumer,
}

func checkResponseResource(
	checked *model.PersistedResource,
	counterpart *model.PersistedResource,
	consumer *model.PersistedResource,
	provider *model.PersistedResource,
) []ContractBreakingChange {
	read := schemagraph.New(consumer.Schemas).Root(consumer.Bodies, schemagraph.Readers)
	provided := schemagraph.New(provider.Schemas).Root(provider.Bodies, schemagraph.Readers)

	return compareBodies(checked, counterpart, responseRules, read, provided)
}
