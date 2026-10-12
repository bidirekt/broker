package compatibility_checker

import (
	"strings"

	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type BreakingReason string

const (
	ReasonProviderResourceNotFound                        BreakingReason = "provider_resource_not_found"
	ReasonPropertyMissingInProvider                       BreakingReason = "property_missing_in_provider"
	ReasonPropertyMissingInConsumer                       BreakingReason = "property_missing_in_consumer"
	ReasonPropertyTypeMismatch                            BreakingReason = "property_type_mismatch"
	ReasonPropertyOptionalInProviderRequiredInConsumer    BreakingReason = "property_optional_in_provider_required_in_consumer"
	ReasonPropertyOptionalInConsumerRequiredInProvider    BreakingReason = "property_optional_in_consumer_required_in_provider"
	ReasonPropertyNullableInProviderNotNullableInConsumer BreakingReason = "property_nullable_in_provider_not_nullable_in_consumer"
	ReasonPropertyNullableInConsumerNotNullableInProvider BreakingReason = "property_nullable_in_consumer_not_nullable_in_provider"
	ReasonProviderResourceNotDeployedInEnvironment        BreakingReason = "provider_resource_not_deployed_in_environment"
	ReasonProviderResourceRemovedButStillConsumed         BreakingReason = "provider_resource_removed_but_still_consumed"
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

func (this *ContractBreakingChange) ConsumerResource() *model.PersistedResource {
	if this.CheckedResource.IsConsumer() {
		return this.CheckedResource
	}

	return this.CounterpartResource
}

func (this *ContractBreakingChange) ConsumerName() string {
	return this.ConsumerResource().ParticipantName
}

func (this *ContractBreakingChange) ProviderName() string {
	return this.ConsumerResource().ConsumedProvider.String
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

// consumerNode and providerNode are where each side stands at propertyPath; either may not exist.
func NewPropertyBreakChange(
	checkedResource *model.PersistedResource,
	counterpartResource *model.PersistedResource,
	reason BreakingReason,
	propertyPath string,
	consumerNode, providerNode schemagraph.Node,
) ContractBreakingChange {
	consumerResource := checkedResource
	if !checkedResource.IsConsumer() {
		consumerResource = counterpartResource
	}

	details := map[string]string{
		"property":     propertyPath,
		"consumerName": consumerResource.ParticipantName,
		"providerName": consumerResource.ConsumedProvider.String,
	}

	if reason == ReasonPropertyTypeMismatch {
		details["consumerPropertyType"] = typeToken(consumerNode)
		details["providerPropertyType"] = typeToken(providerNode)
	} else {
		typed := consumerNode
		if !consumerNode.Exists() {
			typed = providerNode
		}
		details["propertyType"] = typeToken(typed)
	}

	return ContractBreakingChange{
		CheckedResource:     checkedResource,
		CounterpartResource: counterpartResource,
		Reason:              reason,
		Details:             details,
	}
}

// A shape met again while rendering renders as its bare type, so a self-referencing array ends.
func typeToken(node schemagraph.Node) string {
	return typeTokenOf(node, map[string]bool{})
}

func typeTokenOf(node schemagraph.Node, rendering map[string]bool) string {
	shape := node.ShapeKey()
	if rendering[shape] {
		return node.Type()
	}

	rendering[shape] = true

	if node.Type() == "array" {
		return "array<" + typeTokenOf(node.Child(model.ItemsSegment), rendering) + ">"
	}

	if values := node.Child(model.ValuesSegment); values.Exists() && node.Type() == "object" {
		return "map<" + typeTokenOf(values, rendering) + ">"
	}

	return node.Type()
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
