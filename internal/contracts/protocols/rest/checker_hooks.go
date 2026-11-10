package rest

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

func (this Plugin) NewReportTree() compatibility_checker.ReportTree {
	return restmodel.ReportTree{}
}

func (this Plugin) RefineNotFound(hint any) (model.BreakingReason, map[string]string, bool) {
	return model.ReasonProviderResourceInteractionNotFound, providerStatusesDetails(hint.(restmodel.DeployedStatuses)), true
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

	return model.ReasonProviderResourceInteractionRemovedButStillConsumed, providerStatusesDetails(statuses), true
}

func providerStatusesDetails(statuses []string) map[string]string {
	if len(statuses) == 0 {
		return nil
	}

	return map[string]string{"providerStatuses": strings.Join(statuses, ", ")}
}
