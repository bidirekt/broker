package breakreport

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

func toEndpointBreak(checked checkedInteraction, contractBreak storedBreak) EndpointBreak {
	sentence, fixes := endpointBreakLine(checked, contractBreak)

	return EndpointBreak{
		Reason:     contractBreak.Reason,
		Sentence:   sentence,
		ReasonLine: reasonLine(checked, contractBreak),
		Fixes:      fixes,
	}
}

func endpointBreakLine(checked checkedInteraction, contractBreak storedBreak) (Phrase, []Fix) {
	me, other := checked.participant, checked.counterpart.Name
	resource := checked.resource()
	status := checked.interaction
	reason, details := contractBreak.Reason, contractBreak.Details
	asConsumer, asProvider := contractBreak.Role == model.RoleConsumer, contractBreak.Role == model.RoleProvider
	onRequest, onResponse := checked.isRequest(), !checked.isRequest()

	fixes := func(checkedAction, otherAction string) []Fix {
		return []Fix{
			{Participant: me, Action: checkedAction, Checked: true},
			{Participant: other, Action: otherAction},
		}
	}

	switch {
	case reason == model.ReasonProviderResourceNotFound && asConsumer:
		return Phrase{name(me), plain(" calls "), code(resource), plain(", but "), name(other), plain(" doesn't provide it")},
			fixes("stop calling it", "publish it")
	case reason == model.ReasonProviderResourceInteractionNotFound && asConsumer && onRequest:
		return Phrase{name(me), plain(" sends a request body, but "), name(other), plain(" declares none")},
			fixes("stop sending it", "accept a request body")
	case reason == model.ReasonProviderResourceInteractionNotFound && asConsumer && onResponse && details["providerStatuses"] == "":
		return Phrase{name(me), plain(" reads response " + status + ", but "), name(other), plain(" declares no response")},
			fixes("stop reading it", "answer "+status)
	case reason == model.ReasonProviderResourceInteractionNotFound && asConsumer && onResponse:
		providerStatuses := orList(details["providerStatuses"])
		return Phrase{name(me), plain(" reads response " + status + ", but "), name(other), plain(" answers only " + providerStatuses)},
			fixes("read response "+providerStatuses, "answer "+status)
	case reason == model.ReasonProviderResourceNotDeployedInEnvironment && asConsumer:
		return Phrase{name(me), plain(" calls "), code(resource), plain(", but "), name(other), plain(" is not deployed in "), name(checked.environment)},
			[]Fix{{Participant: other, Action: "deploy it to " + checked.environment + " first"}}
	case reason == model.ReasonProviderResourceRemovedButStillConsumed && asProvider:
		return Phrase{name(me), plain(" removed "), code(resource), plain(", but "), name(other), plain(" still calls it")},
			fixes("keep it until "+other+" stops calling it", "stop calling it")
	case reason == model.ReasonProviderResourceInteractionRemovedButStillConsumed && asProvider && onRequest:
		return Phrase{name(me), plain(" dropped the request body of "), code(resource), plain(", but "), name(other), plain(" still sends it")},
			fixes("keep accepting it until "+other+" stops sending it", "stop sending it")
	case reason == model.ReasonProviderResourceInteractionRemovedButStillConsumed && asProvider && onResponse && details["providerStatuses"] == "":
		return Phrase{name(me), plain(" no longer answers "), code(resource), plain(", but "), name(other), plain(" still reads response " + status)},
			fixes("keep answering "+status+" until "+other+" stops reading it", "stop reading it")
	case reason == model.ReasonProviderResourceInteractionRemovedButStillConsumed && asProvider && onResponse:
		providerStatuses := orList(details["providerStatuses"])
		return Phrase{name(me), plain(" now answers "), code(resource), plain(" only with " + providerStatuses + ", but "), name(other), plain(" still reads response " + status)},
			fixes("keep answering "+status+" until "+other+" stops reading it", "read response "+providerStatuses)
	default:
		return Phrase{plain(fallbackBreakLine(reason, details))}, nil
	}
}

func reasonLine(checked checkedInteraction, contractBreak storedBreak) string {
	counterpart, environment := checked.counterpart, checked.environment
	deployedCounterpart := strings.TrimSpace(counterpart.Name + " " + counterpart.Version)

	switch contractBreak.Reason {
	case model.ReasonProviderResourceNotDeployedInEnvironment:
		line := "not deployed in " + environment
		if environments := contractBreak.Details["deployedEnvironments"]; environments != "" {
			line += " (deployed in: " + environments + ")"
		}
		return line
	case model.ReasonProviderResourceNotFound:
		switch {
		case counterpart.Version != "":
			return deployedCounterpart + ", deployed in " + environment + ", doesn't provide this endpoint"
		case counterpart.Registered:
			return counterpart.Name + " is not deployed in " + environment + ", and no version it published provides this endpoint"
		default:
			return counterpart.Name + " is not a participant of this broker"
		}
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
