package rest

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
)

func toEndpointBreak(checked checkedInteraction, contractBreak model.ReportBreak) EndpointBreak {
	sentence, fixes := endpointBreakLine(checked, contractBreak)

	return EndpointBreak{
		Reason:     contractBreak.Reason,
		Sentence:   sentence,
		ReasonLine: reasonLine(checked, contractBreak),
		Fixes:      fixes,
	}
}

func endpointBreakLine(checked checkedInteraction, contractBreak model.ReportBreak) (breakreport.Phrase, []breakreport.Fix) {
	me, other := checked.Participant, checked.Counterpart.Name
	resource := checked.resource()
	status := checked.interaction
	reason, details := contractBreak.Reason, contractBreak.Details
	asConsumer, asProvider := contractBreak.Role == model.RoleConsumer, contractBreak.Role == model.RoleProvider
	onRequest, onResponse := checked.isRequest(), !checked.isRequest()

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
	case reason == model.ReasonProviderResourceInteractionNotFound && asConsumer && onRequest:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" sends a request body, but "), breakreport.Name(other), breakreport.Plain(" declares none")},
			fixes("stop sending it", "accept a request body")
	case reason == model.ReasonProviderResourceInteractionNotFound && asConsumer && onResponse && details["providerStatuses"] == "":
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" reads response " + status + ", but "), breakreport.Name(other), breakreport.Plain(" declares no response")},
			fixes("stop reading it", "answer "+status)
	case reason == model.ReasonProviderResourceInteractionNotFound && asConsumer && onResponse:
		providerStatuses := orList(details["providerStatuses"])
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" reads response " + status + ", but "), breakreport.Name(other), breakreport.Plain(" answers only " + providerStatuses)},
			fixes("read response "+providerStatuses, "answer "+status)
	case reason == model.ReasonProviderResourceNotDeployedInEnvironment && asConsumer:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" calls "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" is not deployed in "), breakreport.Name(checked.Environment)},
			[]breakreport.Fix{{Participant: other, Action: "deploy it to " + checked.Environment + " first"}}
	case reason == model.ReasonProviderResourceRemovedButStillConsumed && asProvider:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" removed "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" still calls it")},
			fixes("keep it until "+other+" stops calling it", "stop calling it")
	case reason == model.ReasonProviderResourceInteractionRemovedButStillConsumed && asProvider && onRequest:
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" dropped the request body of "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" still sends it")},
			fixes("keep accepting it until "+other+" stops sending it", "stop sending it")
	case reason == model.ReasonProviderResourceInteractionRemovedButStillConsumed && asProvider && onResponse && details["providerStatuses"] == "":
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" no longer answers "), breakreport.Code(resource), breakreport.Plain(", but "), breakreport.Name(other), breakreport.Plain(" still reads response " + status)},
			fixes("keep answering "+status+" until "+other+" stops reading it", "stop reading it")
	case reason == model.ReasonProviderResourceInteractionRemovedButStillConsumed && asProvider && onResponse:
		providerStatuses := orList(details["providerStatuses"])
		return breakreport.Phrase{breakreport.Name(me), breakreport.Plain(" now answers "), breakreport.Code(resource), breakreport.Plain(" only with " + providerStatuses + ", but "), breakreport.Name(other), breakreport.Plain(" still reads response " + status)},
			fixes("keep answering "+status+" until "+other+" stops reading it", "read response "+providerStatuses)
	default:
		return breakreport.Phrase{breakreport.Plain(fallbackBreakLine(reason, details))}, nil
	}
}

func reasonLine(checked checkedInteraction, contractBreak model.ReportBreak) string {
	counterpart, environment := checked.Counterpart, checked.Environment
	deployedCounterpart := strings.TrimSpace(counterpart.Name + " " + counterpart.Version)

	switch contractBreak.Reason {
	case model.ReasonProviderResourceNotDeployedInEnvironment:
		return "not deployed in " + environment
	case model.ReasonProviderResourceNotFound:
		return deployedCounterpart + ", deployed in " + environment + ", doesn't provide this endpoint"
	case model.ReasonProviderResourceInteractionNotFound:
		line := deployedCounterpart + ", deployed in " + environment + ", declares the endpoint but not this interaction"
		if statuses := contractBreak.Details["providerStatuses"]; statuses != "" {
			line += " (declared: " + statuses + ")"
		}
		return line
	default:
		return ""
	}
}

func orList(commaList string) string {
	last := strings.LastIndex(commaList, ", ")
	if last < 0 {
		return commaList
	}
	return commaList[:last] + " or " + commaList[last+len(", "):]
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
