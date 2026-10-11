package compatibility_checker

import "github.com/bidirekt/broker/internal/model"

func checkRequestResource(
	checked *model.PersistedResource,
	counterpart *model.PersistedResource,
	consumer *model.PersistedResource,
	provider *model.PersistedResource,
) []ContractBreakingChange {
	var breaks []ContractBreakingChange

	for providerPropertyPath, providerProperty := range provider.Properties {
		consumerProperty, propertyExists := consumer.Properties[providerPropertyPath]
		// If the property is not present in the consumer, is not optional and its parent is present in the consumer, it is a breaking change.
		if !propertyExists {
			if !providerProperty.Optional && parentPresentIn(consumer, providerPropertyPath) {
				breaks = append(breaks, NewPropertyBreakChange(
					checked,
					counterpart,
					ReasonPropertyMissingInConsumer,
					providerPropertyPath,
				))
			}

			continue
		}

		// If the property is present in the consumer with a type the provider does not accept, it is a breaking change.
		if !acceptsType(providerProperty.Type, consumerProperty.Type) {
			breaks = append(breaks, NewPropertyBreakChange(
				checked,
				counterpart,
				ReasonPropertyTypeMismatch,
				providerPropertyPath,
			))

			continue
		}

		// If the property is required in the provider and is optional in the consumer, it is a breaking change.
		if !providerProperty.Optional && consumerProperty.Optional {
			breaks = append(breaks, NewPropertyBreakChange(
				checked,
				counterpart,
				ReasonPropertyOptionalInConsumerRequiredInProvider,
				providerPropertyPath,
			))
		}

		if consumerProperty.Nullable && !providerProperty.Nullable {
			breaks = append(breaks, NewPropertyBreakChange(
				checked,
				counterpart,
				ReasonPropertyNullableInConsumerNotNullableInProvider,
				providerPropertyPath,
			))
		}
	}

	return breaks
}
