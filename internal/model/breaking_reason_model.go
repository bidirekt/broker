package model

import "strings"

type BreakingReason string

const (
	ReasonProviderResourceNotFound                           BreakingReason = "provider_resource_not_found"
	ReasonProviderResourceInteractionNotFound                BreakingReason = "provider_resource_interaction_not_found"
	ReasonPropertyMissingInProvider                          BreakingReason = "property_missing_in_provider"
	ReasonPropertyMissingInConsumer                          BreakingReason = "property_missing_in_consumer"
	ReasonPropertyMemberMissingInProvider                    BreakingReason = "property_member_missing_in_provider"
	ReasonPropertyTypeMismatch                               BreakingReason = "property_type_mismatch"
	ReasonPropertyOptionalInProviderRequiredInConsumer       BreakingReason = "property_optional_in_provider_required_in_consumer"
	ReasonPropertyOptionalInConsumerRequiredInProvider       BreakingReason = "property_optional_in_consumer_required_in_provider"
	ReasonPropertyNullableInProviderNotNullableInConsumer    BreakingReason = "property_nullable_in_provider_not_nullable_in_consumer"
	ReasonPropertyNullableInConsumerNotNullableInProvider    BreakingReason = "property_nullable_in_consumer_not_nullable_in_provider"
	ReasonProviderResourceNotDeployedInEnvironment           BreakingReason = "provider_resource_not_deployed_in_environment"
	ReasonProviderResourceRemovedButStillConsumed            BreakingReason = "provider_resource_removed_but_still_consumed"
	ReasonProviderResourceInteractionRemovedButStillConsumed BreakingReason = "provider_resource_interaction_removed_but_still_consumed"
)

type BreakRole string

const (
	RoleConsumer BreakRole = "consumer"
	RoleProvider BreakRole = "provider"
)

func (this BreakingReason) IsPropertyBreak() bool {
	return strings.HasPrefix(string(this), "property_")
}
