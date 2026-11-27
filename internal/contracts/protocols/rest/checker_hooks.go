package rest

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

func (this Plugin) NewReportTree() compatibility_checker.ReportTree {
	return restmodel.ReportTree{}
}

var bodyReadings = map[model.BodyRole]compatibility_checker.BodyReading{
	model.Input: {
		Rules: compatibility_checker.ReadingRules{
			ReaderIsConsumer: false,
			Missing:          model.ReasonPropertyMissingInConsumer,
			Optional:         model.ReasonPropertyOptionalInConsumerRequiredInProvider,
			Nullable:         model.ReasonPropertyNullableInConsumerNotNullableInProvider,
		},
		Join: schemagraph.Senders,
	},
	model.Output: {
		Rules: compatibility_checker.ReadingRules{
			ReaderIsConsumer: true,
			Missing:          model.ReasonPropertyMissingInProvider,
			Optional:         model.ReasonPropertyOptionalInProviderRequiredInConsumer,
			Nullable:         model.ReasonPropertyNullableInProviderNotNullableInConsumer,
		},
		Join: schemagraph.Readers,
	},
}

func (this Plugin) BodyReading(body model.BodyRole) compatibility_checker.BodyReading {
	return bodyReadings[body]
}

func (this Plugin) RefineNotFound(hint any) (model.BreakingReason, map[string]string, bool) {
	return restmodel.ReasonProviderResourceInteractionNotFound, providerStatusesDetails(hint.(restmodel.DeployedStatuses)), true
}

// A removed request keeps no statuses: its endpoint and method only remain through responses.
func (this Plugin) RefineRemoved(
	removed *model.PersistedResource,
	checkedResources map[string]model.PersistedResource,
) (model.BreakingReason, map[string]string, bool) {
	removedIdentity := removed.Identity.(restmodel.Identity)

	var statuses []string
	methodRemains := false

	for _, resource := range checkedResources {
		if resource.Removed || !resource.IsProvider() || resource.Protocol != restmodel.Protocol {
			continue
		}

		identity := resource.Identity.(restmodel.Identity)
		if identity.Endpoint != removedIdentity.Endpoint || identity.Method != removedIdentity.Method {
			continue
		}

		methodRemains = true

		if removed.Body == model.Output && resource.Body == model.Output {
			statuses = append(statuses, identity.Status)
		}
	}

	if !methodRemains {
		return "", nil, false
	}

	slices.Sort(statuses)

	return restmodel.ReasonProviderResourceInteractionRemovedButStillConsumed, providerStatusesDetails(statuses), true
}

func providerStatusesDetails(statuses []string) map[string]string {
	if len(statuses) == 0 {
		return nil
	}

	return map[string]string{"providerStatuses": strings.Join(statuses, ", ")}
}
