package compatibility_checker

import (
	"context"
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/model"
)

// checkExtraCounterparts runs outside the pair cache: an extra counterpart has no contract of its own, so its
// breaks are always live. It answers the providers each protocol took over.
func (this *CompatibilityChecker) checkExtraCounterparts(
	ctx context.Context,
	contract *model.PersistedContract,
	environment *model.Environment,
	report *ContractCompatibilityReport,
) map[model.Protocol]map[string]bool {
	assumed := map[model.Protocol]map[string]bool{}

	for _, hooks := range this.protocols {
		port, extra := hooks.(ExtraCounterparts)
		if !extra {
			continue
		}

		pairs := port.ExtraCounterparts(ctx, contract, environment)
		assumed[hooks.Protocol()] = pairs.Providers

		for _, pair := range pairs.Pairs {
			report.AppendResult(pair.Counterpart.ParticipantName, this.checkExtraPair(pair))
		}
	}

	return assumed
}

func (this *CompatibilityChecker) checkExtraPair(pair ExtraPair) *CounterpartResult {
	result := NewCounterpartResult()
	result.Counterpart = pair.Counterpart
	role := sideUnderCheck(pair.Counterpart.Role)

	for index := range pair.Resources {
		consumed := &pair.Resources[index]

		breaks := this.compareExtra(consumed, pair.Contract, pair.ReadyBreak, role)
		if pair.Base != nil {
			breaks = slices.DeleteFunc(breaks, containedIn(this.compareExtra(consumed, pair.Base, ReadyBreak{}, role)))
		}

		for _, breakingChange := range breaks {
			breakingChange.Role = role
			breakingChange.Details = withDetails(breakingChange.Details, pair.Details)
			result.AppendLiveBreak(breakingChange)
		}
	}

	return result
}

// A consumed resource the provider lacks is not found when the consumer is under check, and removed when the
// provider is: the base already discounts the ones it never had.
func (this *CompatibilityChecker) compareExtra(
	consumed *model.PersistedResource,
	provider *model.PersistedContract,
	readyBreak ReadyBreak,
	role model.BreakRole,
) []ContractBreakingChange {
	if provider == nil {
		return []ContractBreakingChange{{CheckedResource: consumed, Reason: readyBreak.Reason, Details: readyBreak.Details}}
	}

	provided, found := provider.Resources[consumed.ProviderHash]

	switch {
	case !found && role == model.RoleConsumer:
		return []ContractBreakingChange{this.toProviderResourceNotFound(consumed, nil)}
	case !found:
		return []ContractBreakingChange{{CheckedResource: consumed, Reason: model.ReasonProviderResourceRemovedButStillConsumed}}
	case role == model.RoleConsumer:
		return this.checkResources(consumed, &provided)
	default:
		return this.checkResources(&provided, consumed)
	}
}

func containedIn(breaks []ContractBreakingChange) func(ContractBreakingChange) bool {
	return func(candidate ContractBreakingChange) bool {
		return slices.ContainsFunc(breaks, func(breakingChange ContractBreakingChange) bool {
			return breakingChange.Reason == candidate.Reason && maps.Equal(breakingChange.Details, candidate.Details)
		})
	}
}

func withDetails(details, added map[string]string) map[string]string {
	if len(added) == 0 {
		return details
	}

	merged := maps.Clone(details)
	if merged == nil {
		merged = map[string]string{}
	}

	maps.Copy(merged, added)

	return merged
}

func sideUnderCheck(counterpartRole model.CounterpartRole) model.BreakRole {
	if counterpartRole == model.CounterpartProvider {
		return model.RoleConsumer
	}

	return model.RoleProvider
}
