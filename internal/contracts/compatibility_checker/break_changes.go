package compatibility_checker

import (
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
)

type ContractBreakingChange struct {
	CheckedResource     *model.PersistedResource
	CounterpartResource *model.PersistedResource
	Reason              model.BreakingReason
	Details             map[string]string
}

func (this *ContractBreakingChange) refine(reason model.BreakingReason, details map[string]string, refined bool) {
	if !refined {
		return
	}

	this.Reason = reason
	this.Details = details
}

func (this *ContractBreakingChange) IsPropertyBreak() bool {
	return this.Reason.IsPropertyBreak()
}

func (this *ContractBreakingChange) ConsumerResource() *model.PersistedResource {
	consumer, _ := consumerAndProvider(this.CheckedResource, this.CounterpartResource)

	return consumer
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
	reason model.BreakingReason,
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
		Reason:          model.ReasonProviderResourceNotFound,
	}
}

func NewProviderResourceRemovedButStillConsumed(
	removedResource *model.PersistedResource,
	consumerResource *model.PersistedResource,
) ContractBreakingChange {
	return ContractBreakingChange{
		CheckedResource:     removedResource,
		CounterpartResource: consumerResource,
		Reason:              model.ReasonProviderResourceRemovedButStillConsumed,
	}
}

// consumerNode and providerNode are where each side stands at propertyPath; either may not exist.
func NewPropertyBreakChange(
	checkedResource *model.PersistedResource,
	counterpartResource *model.PersistedResource,
	reason model.BreakingReason,
	propertyPath string,
	consumerNode, providerNode schemagraph.Node,
) ContractBreakingChange {
	consumerResource, _ := consumerAndProvider(checkedResource, counterpartResource)

	details := map[string]string{
		"property":     propertyPath,
		"consumerName": consumerResource.ParticipantName,
		"providerName": consumerResource.ConsumedProvider.String,
	}

	if consumerNode.Exists() {
		details["consumerField"] = consumerNode.Field()
	}

	if providerNode.Exists() {
		details["providerField"] = providerNode.Field()
	}

	if reason == model.ReasonPropertyTypeMismatch {
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

	if node.Type() == "map" {
		return "map<" + typeTokenOf(node.Child(model.ValuesSegment), rendering) + ">"
	}

	return node.Type()
}

func NewProviderResourceNotDeployed(consumerResource *model.PersistedResource) ContractBreakingChange {
	return ContractBreakingChange{
		CheckedResource: consumerResource,
		Reason:          model.ReasonProviderResourceNotDeployedInEnvironment,
	}
}
