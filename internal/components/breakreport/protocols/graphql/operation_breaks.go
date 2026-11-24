package graphql

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
)

func toOperationBreak(checked checkedOperation, contractBreak model.ReportBreak) OperationBreak {
	sentence, fixes := operationBreakLine(checked, contractBreak)

	broken := OperationBreak{Reason: contractBreak.Reason, Sentence: sentence, Fixes: fixes}
	if contractBreak.Reason.IsPropertyBreak() {
		broken.Interaction = checked.interaction
	}

	return broken
}

func operationBreakLine(checked checkedOperation, contractBreak model.ReportBreak) (breakreport.Phrase, []breakreport.Fix) {
	me, other := checked.Participant, checked.Counterpart.Name
	resource := checked.resource()
	reason := contractBreak.Reason
	asConsumer, asProvider := contractBreak.Role == model.RoleConsumer, contractBreak.Role == model.RoleProvider

	fixes := func(checkedAction, otherAction string) []breakreport.Fix {
		return []breakreport.Fix{
			{Participant: me, Action: checkedAction, Checked: true},
			{Participant: other, Action: otherAction},
		}
	}

	switch {
	case reason == model.ReasonProviderResourceNotFound && asConsumer:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" calls "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" doesn't provide it")},
			fixes("stop calling it", "publish it")
	case reason == model.ReasonProviderResourceNotDeployedInEnvironment && asConsumer:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" calls "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" is not deployed in "), breakreport.Name(checked.Environment)},
			[]breakreport.Fix{{Participant: other, Action: "deploy it to " + checked.Environment + " first"}}
	case reason == model.ReasonProviderResourceRemovedButStillConsumed && asProvider:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" removed "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" still calls it")},
			fixes("keep it until "+other+" stops calling it", "stop calling it")
	default:
		return breakreport.Phrase{breakreport.Plain(fallbackBreakLine(reason, contractBreak.Details))}, nil
	}
}

func fallbackBreakLine(reason model.BreakingReason, details map[string]string) string {
	if len(details) == 0 {
		return string(reason)
	}

	pairs := make([]string, 0, len(details))
	for _, key := range slices.Sorted(maps.Keys(details)) {
		pairs = append(pairs, key+": "+details[key])
	}

	return fmt.Sprintf("%s (%s)", reason, strings.Join(pairs, ", "))
}
