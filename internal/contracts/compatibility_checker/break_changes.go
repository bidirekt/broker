package compatibility_checker

import (
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

type BreakingReason string

const (
	ReasonProviderResourceNotFound                     BreakingReason = "provider_resource_not_found"
	ReasonPropertyMissingInProvider                    BreakingReason = "property_missing_in_provider"
	ReasonPropertyMissingInConsumer                    BreakingReason = "property_missing_in_consumer"
	ReasonPropertyTypeMismatch                         BreakingReason = "property_type_mismatch"
	ReasonPropertyOptionalInProviderRequiredInConsumer BreakingReason = "property_optional_in_provider_required_in_consumer"
	ReasonPropertyOptionalInConsumerRequiredInProvider BreakingReason = "property_optional_in_consumer_required_in_provider"
	ReasonProviderResourceNotDeployedInEnvironment     BreakingReason = "provider_resource_not_deployed_in_environment"
	ReasonProviderResourceRemovedButStillConsumed      BreakingReason = "provider_resource_removed_but_still_consumed"
)

type BreakRole string

const (
	RoleConsumer BreakRole = "consumer"
	RoleProvider BreakRole = "provider"
)

type ContractBreakingChange struct {
	CheckedResource     *model.PersistedResource `json:"-"`
	CounterpartResource *model.PersistedResource `json:"-"`
	Reason              BreakingReason           `json:"reason"`
	Role                BreakRole                `json:"role"`
	Details             map[string]string        `json:"details,omitempty"`
}

func (b *ContractBreakingChange) ConsumerResource() *model.PersistedResource {
	if b.CheckedResource.IsConsumer() {
		return b.CheckedResource
	}

	return b.CounterpartResource
}

func (b *ContractBreakingChange) ConsumerName() string {
	return b.ConsumerResource().ParticipantName
}

func (b *ContractBreakingChange) ProviderName() string {
	return b.ConsumerResource().ConsumedProvider.String
}

func NewContractBreakingChange(
	checkedResource *model.PersistedResource,
	counterpartResource *model.PersistedResource,
	reason BreakingReason,
) ContractBreakingChange {
	return ContractBreakingChange{
		CheckedResource:     checkedResource,
		CounterpartResource: counterpartResource,
		Reason:              reason,
	}
}

func NewProviderResourceNotFound(checkedResource *model.PersistedResource) ContractBreakingChange {
	return ContractBreakingChange{
		CheckedResource: checkedResource,
		Reason:          ReasonProviderResourceNotFound,
	}
}

func NewProviderResourceRemovedButStillConsumed(
	removedResource *model.PersistedResource,
	consumerResource *model.PersistedResource,
) ContractBreakingChange {
	return ContractBreakingChange{
		CheckedResource:     removedResource,
		CounterpartResource: consumerResource,
		Reason:              ReasonProviderResourceRemovedButStillConsumed,
	}
}

func NewPropertyBreakChange(
	checkedResource *model.PersistedResource,
	counterpartResource *model.PersistedResource,
	reason BreakingReason,
	propertyPath string,
) ContractBreakingChange {
	consumerResource, providerResource := checkedResource, counterpartResource
	if !checkedResource.IsConsumer() {
		consumerResource, providerResource = counterpartResource, checkedResource
	}

	details := map[string]string{
		"property":     propertyPath,
		"consumerName": consumerResource.ParticipantName,
		"providerName": consumerResource.ConsumedProvider.String,
	}

	if reason == ReasonPropertyTypeMismatch {
		details["consumerPropertyType"] = typeToken(consumerResource, propertyPath)
		details["providerPropertyType"] = typeToken(providerResource, propertyPath)
	} else {
		typed := consumerResource
		if _, exists := consumerResource.Properties[propertyPath]; !exists {
			typed = providerResource
		}
		details["propertyType"] = typeToken(typed, propertyPath)
	}

	return ContractBreakingChange{
		CheckedResource:     checkedResource,
		CounterpartResource: counterpartResource,
		Reason:              reason,
		Details:             details,
	}
}

// typeToken renders a property's type, resolving array item types from the
// flat property map: $.list (array) + $.list[] (string) → "array<string>".
func typeToken(resource *model.PersistedResource, propertyPath string) string {
	if resource.Properties[propertyPath].Type == "array" {
		return "array<" + typeToken(resource, propertyPath+"[]") + ">"
	}

	return resource.Properties[propertyPath].Type
}

func NewProviderResourceNotDeployed(
	consumerResource *model.PersistedResource,
	deployedEnvironments []string,
) ContractBreakingChange {
	var details map[string]string

	if len(deployedEnvironments) > 0 {
		details = map[string]string{
			"deployedEnvironments": strings.Join(deployedEnvironments, ", "),
		}
	}

	return ContractBreakingChange{
		CheckedResource: consumerResource,
		Reason:          ReasonProviderResourceNotDeployedInEnvironment,
		Details:         details,
	}
}
