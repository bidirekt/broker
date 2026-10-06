package compatibility_checker

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
)

type VerdictReader interface {
	GetVerdict(ctx context.Context, one, two int64) (*model.CompatibilityVerdict, bool)
}

type CompatibilityChecker struct {
	verdicts VerdictReader
}

func NewCompatibilityChecker(verdicts VerdictReader) *CompatibilityChecker {
	return &CompatibilityChecker{verdicts: verdicts}
}

func (c *CompatibilityChecker) Check(
	ctx context.Context,
	contract *model.PersistedContract,
	environment *model.Environment,
	counterparts model.ResourceCounterparts,
) *ContractCompatibilityReport {
	report := NewContractCompatibilityReport(
		contract.ParticipantName,
		contract.Version,
		environment.Name,
	)

	pairs := &checkedPairs{
		verdicts:   c.verdicts,
		contractID: contract.ID,
		states:     make(map[[2]int64]*pairState),
	}

	for _, resource := range contract.Resources {
		if resource.Removed {
			if resource.Direction == model.Provides {
				c.checkRemovedProvider(resource, counterparts, report)
			}

			continue
		}

		switch resource.Direction {
		case model.Consumes:
			c.checkConsumer(ctx, resource, environment, counterparts, pairs, report)
		case model.Provides:
			c.checkProvider(ctx, resource, counterparts, pairs, report)
		}
	}

	return report
}

// checkedPairs is the per-call memoization state: one entry per canonical snapshot pair,
// resolved on the pair's first occurrence. Snapshots are immutable, so nothing outlives the
// call and there is nothing to invalidate.
type checkedPairs struct {
	verdicts   VerdictReader
	contractID int64
	states     map[[2]int64]*pairState
}

type pairState struct {
	verdict  *model.CompatibilityVerdict
	injected bool
}

func (p *checkedPairs) resolve(ctx context.Context, counterpartContractID int64) *pairState {
	one, two := model.OrderContractPair(p.contractID, counterpartContractID)

	state, seen := p.states[[2]int64{one, two}]
	if seen {
		return state
	}

	state = &pairState{}
	if verdict, found := p.verdicts.GetVerdict(ctx, one, two); found {
		state.verdict = verdict
	}

	p.states[[2]int64{one, two}] = state

	return state
}

func (s *pairState) hit() bool {
	return s.verdict != nil
}

// replay injects the stored breaks once per pair: a verdict already aggregates every resource
// of the pair, so the remaining resources contribute nothing.
func (s *pairState) replay(item *IncompatibleItem) {
	if s.injected {
		return
	}

	s.injected = true

	for _, verdictBreak := range s.verdict.Breaks {
		item.AppendCachedBreakChange(verdictBreak)
	}
}
