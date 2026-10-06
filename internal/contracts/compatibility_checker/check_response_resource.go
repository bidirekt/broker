package compatibility_checker

import "github.com/bidirekt/broker/internal/model"

func checkResponseResource(
	checked *model.PersistedResource,
	counterpart *model.PersistedResource,
	consumer *model.PersistedResource,
	provider *model.PersistedResource,
) []ContractBreakingChange {
	var breaks []ContractBreakingChange

	for consumerPropertyPath, consumerProperty := range consumer.Properties {
		providerProperty, propertyExists := provider.Properties[consumerPropertyPath]

		// If the property is not present in the provider and is required in the consumer, it is a breaking change.
		if !propertyExists {
			if !consumerProperty.Optional {
				breaks = append(breaks, NewPropertyBreakChange(
					checked,
					counterpart,
					ReasonPropertyMissingInProvider,
					consumerPropertyPath,
				))
			}

			continue
		}

		// If the property is present in the provider and the type is different, it is a breaking change.
		if consumerProperty.Type != providerProperty.Type {
			breaks = append(breaks, NewPropertyBreakChange(
				checked,
				counterpart,
				ReasonPropertyTypeMismatch,
				consumerPropertyPath,
			))

			continue
		}

		// If the property is required in the consumer and is optional in the provider, it is a breaking change.
		if !consumerProperty.Optional && providerProperty.Optional {
			breaks = append(breaks, NewPropertyBreakChange(
				checked,
				counterpart,
				ReasonPropertyOptionalInProviderRequiredInConsumer,
				consumerPropertyPath,
			))
		}
	}

	return breaks
}
