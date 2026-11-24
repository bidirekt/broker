package rest

import "github.com/bidirekt/broker/internal/model"

const (
	ReasonProviderResourceInteractionNotFound                model.BreakingReason = "provider_resource_interaction_not_found"
	ReasonProviderResourceInteractionRemovedButStillConsumed model.BreakingReason = "provider_resource_interaction_removed_but_still_consumed"
)
