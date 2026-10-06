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
		// If the property is not present in the consumer and is not optional, it is a breaking change.
		if !propertyExists {
			if !providerProperty.Optional {
				breaks = append(breaks, NewPropertyBreakChange(
					checked,
					counterpart,
					ReasonPropertyMissingInConsumer,
					providerPropertyPath,
				))
			}

			continue
		}

		// If the property is present in the consumer and the type is different, it is a breaking change.
		if consumerProperty.Type != providerProperty.Type {
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
	}

	return breaks
}
