package graphql

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func toOperationBreak(checked checkedOperation, contractBreak model.ReportBreak) OperationBreak {
	sentence, fixes := operationBreakLine(checked, contractBreak)

	broken := OperationBreak{Reason: contractBreak.Reason, Sentence: sentence, Fixes: fixes}
	if contractBreak.Reason.IsPropertyBreak() {
		broken.Interaction = checked.interaction
	}

	if contractBreak.Reason == graphqlmodel.ReasonProviderGraphNotComposable {
		broken.Graph = contractBreak.Details[graphqlmodel.GraphKey]
		broken.Causes, broken.MoreCauses = shownCauses(compositionCauses(contractBreak.Details))
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

	graph := contractBreak.Details[graphqlmodel.GraphKey]

	subject := breakreport.Phrase{breakreport.Name(me)}
	if asProvider && graph != "" {
		subject = append(subject, viaGraph(graph))
	}

	toDeploy := "it"
	if graph != "" {
		toDeploy = "a subgraph of it"
	}

	switch {
	case reason == graphqlmodel.ReasonProviderGraphNotComposable && asConsumer:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" calls "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" can't be built in "), breakreport.Name(checked.Environment)},
			[]breakreport.Fix{{Participant: other + " subgraphs", Action: "make it compose"}}
	case reason == graphqlmodel.ReasonProviderGraphNotComposable && asProvider:
		return append(subject, breakreport.Plain(" makes "), breakreport.Name(graph), breakreport.Plain(" fail to compose in "), breakreport.Name(checked.Environment), breakreport.Plain(", and "), breakreport.Name(other), breakreport.Plain(" calls "), breakreport.Code(resource)),
			[]breakreport.Fix{{Participant: me, Action: "make " + graph + " compose", Checked: true}}
	case reason == model.ReasonProviderResourceNotFound && asConsumer:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" calls "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" doesn't provide it")},
			fixes("stop calling it", "publish it")
	case reason == model.ReasonProviderResourceNotDeployedInEnvironment && asConsumer:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" calls "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" is not deployed in "), breakreport.Name(checked.Environment)},
			[]breakreport.Fix{{Participant: other, Action: "deploy " + toDeploy + " to " + checked.Environment + " first"}}
	case reason == model.ReasonProviderResourceRemovedButStillConsumed && asProvider:
		return append(subject, breakreport.Plain(" removed "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" still calls it")),
			fixes("keep it until "+other+" stops calling it", "stop calling it")
	default:
		return breakreport.Phrase{breakreport.Plain(fallbackBreakLine(reason, contractBreak.Details))}, nil
	}
}

const maxShownCauses = 5

func shownCauses(causes []string) ([]string, int) {
	if len(causes) <= maxShownCauses {
		return causes, 0
	}

	return causes[:maxShownCauses], len(causes) - maxShownCauses
}

// Checks stored before causes existed carry only the first cause.
func compositionCauses(details map[string]string) []string {
	if causes := details[graphqlmodel.CausesKey]; causes != "" {
		return strings.Split(causes, "\n")
	}

	return []string{details[graphqlmodel.CauseKey]}
}

// A subgraph under check breaks a consumer through the graph it belongs to.
func viaGraph(graph string) breakreport.Segment {
	return breakreport.Plain(" (via " + graph + ")")
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
