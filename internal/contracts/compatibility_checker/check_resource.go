package compatibility_checker

import "github.com/bidirekt/broker/internal/model"

func checkResources(checked *model.PersistedResource, counterpart *model.PersistedResource) []ContractBreakingChange {
	consumer, provider := checked, counterpart
	if !checked.IsConsumer() {
		consumer, provider = counterpart, checked
	}

	switch consumer.Interaction {
	case model.RestRequest:
		return checkRequestResource(checked, counterpart, consumer, provider)
	default:
		return checkResponseResource(checked, counterpart, consumer, provider)
	}
}

func parentPresentIn(resource *model.PersistedResource, path string) bool {
	parent, hasParent := model.ParentPath(path)
	if !hasParent {
		return true
	}

	_, present := resource.Properties[parent]

	return present
}
