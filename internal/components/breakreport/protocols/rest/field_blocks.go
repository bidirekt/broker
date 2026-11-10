package rest

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

// The sentence and the fixes follow from the key and the two names, so blocks that share a
// key also share them.
type fieldBlockKey struct {
	field                                    string
	reason                                   model.BreakingReason
	consumerType, providerType, propertyType string
	member                                   string
	direction                                string
	role                                     model.BreakRole
}

type fieldBlock struct {
	fieldBlockKey
	sentence             breakreport.Phrase
	checkedFix, otherFix breakreport.Fix
	places               []blockPlace
}

type blockPlace struct {
	Place
	interaction, property string
}

func (this fieldBlock) toFieldBlock() FieldBlock {
	places := make([]Place, 0, len(this.places))
	otherFix := this.otherFix
	for _, place := range this.places {
		places = append(places, place.Place)
	}
	if slices.ContainsFunc(places, func(place Place) bool { return place.CounterpartField != "" }) {
		otherFix.Action += " there"
	}

	return FieldBlock{
		Field:    this.field,
		Reason:   this.reason,
		Sentence: this.sentence,
		Fixes:    []breakreport.Fix{this.checkedFix, otherFix},
		Places:   places,
	}
}

type propertyBreak struct {
	consumer, provider           string
	consumerField, providerField string
	path                         string
	consumerIsChecked            bool
}

type propertyBreakPhrases struct {
	consumerChecked, providerChecked breakreport.Phrase
	consumerFix, providerFix         string
}

func (this propertyBreak) checkedAndOtherFields() (checkedField, otherField string) {
	if this.consumerIsChecked {
		return this.consumerField, this.providerField
	}
	return this.providerField, this.consumerField
}

func toFieldBlock(checked checkedInteraction, contractBreak model.ReportBreak) (fieldBlock, bool) {
	reason, details := contractBreak.Reason, contractBreak.Details
	consumerType, providerType := details["consumerPropertyType"], details["providerPropertyType"]
	asConsumer, asProvider := contractBreak.Role == model.RoleConsumer, contractBreak.Role == model.RoleProvider
	consumer, provider := checked.Participant, checked.Counterpart.Name
	if asProvider {
		consumer, provider = provider, consumer
	}
	property := propertyBreak{
		consumer:          consumer,
		provider:          provider,
		consumerField:     details["consumerField"],
		providerField:     details["providerField"],
		path:              details["property"],
		consumerIsChecked: asConsumer,
	}
	member := strconv.Quote(details["member"])
	onRequest, onResponse := checked.isRequest(), !checked.isRequest()

	if !asConsumer && !asProvider || property.path == "" {
		return fieldBlock{}, false
	}

	var phrases propertyBreakPhrases
	switch {
	case reason == model.ReasonPropertyMissingInProvider && onResponse:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" reads it, but "), breakreport.Name(provider), breakreport.Plain(" doesn't provide it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" doesn't provide it, but "), breakreport.Name(consumer), breakreport.Plain(" reads it")},
			consumerFix:     "stop reading it, or handle its absence in the code and mark it optional",
			providerFix:     "provide it",
		}
	case reason == model.ReasonPropertyOptionalInProviderRequiredInConsumer && onResponse:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" requires it, but "), breakreport.Name(provider), breakreport.Plain(" only sometimes provides it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" provides it only sometimes, but "), breakreport.Name(consumer), breakreport.Plain(" requires it")},
			consumerFix:     "handle its absence in the code, then mark it optional",
			providerFix:     "always provide it",
		}
	case reason == model.ReasonPropertyTypeMismatch && onResponse:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" reads it as "), breakreport.Code(consumerType), breakreport.Plain(", but "), breakreport.Name(provider), breakreport.Plain(" provides "), breakreport.Code(providerType)},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" provides it as "), breakreport.Code(providerType), breakreport.Plain(", but "), breakreport.Name(consumer), breakreport.Plain(" reads "), breakreport.Code(consumerType)},
			consumerFix:     "read it as " + providerType,
			providerFix:     "provide it as " + consumerType,
		}
	case reason == model.ReasonPropertyNullableInProviderNotNullableInConsumer && onResponse:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" doesn't accept null, but "), breakreport.Name(provider), breakreport.Plain(" may provide null")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" may provide null, but "), breakreport.Name(consumer), breakreport.Plain(" doesn't accept it")},
			consumerFix:     "handle null in the code, then mark it nullable",
			providerFix:     "stop providing null",
		}
	case reason == model.ReasonPropertyMemberMissingInProvider && onResponse:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" reads " + member + ", but "), breakreport.Name(provider), breakreport.Plain(" doesn't provide it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" doesn't provide " + member + ", but "), breakreport.Name(consumer), breakreport.Plain(" reads it")},
			consumerFix:     "stop reading " + member,
			providerFix:     "provide " + member,
		}
	case reason == model.ReasonPropertyMissingInConsumer && onRequest:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" doesn't send it, but "), breakreport.Name(provider), breakreport.Plain(" requires it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" requires it, but "), breakreport.Name(consumer), breakreport.Plain(" doesn't send it")},
			consumerFix:     "send it",
			providerFix:     "stop requiring it, then mark it optional",
		}
	case reason == model.ReasonPropertyOptionalInConsumerRequiredInProvider && onRequest:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" sends it only sometimes, but "), breakreport.Name(provider), breakreport.Plain(" requires it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" requires it, but "), breakreport.Name(consumer), breakreport.Plain(" sends it only sometimes")},
			consumerFix:     "always send it",
			providerFix:     "handle its absence in the code, then mark it optional",
		}
	case reason == model.ReasonPropertyTypeMismatch && onRequest:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" sends it as "), breakreport.Code(consumerType), breakreport.Plain(", but "), breakreport.Name(provider), breakreport.Plain(" expects "), breakreport.Code(providerType)},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" expects it as "), breakreport.Code(providerType), breakreport.Plain(", but "), breakreport.Name(consumer), breakreport.Plain(" sends "), breakreport.Code(consumerType)},
			consumerFix:     "send " + providerType,
			providerFix:     "accept " + consumerType,
		}
	case reason == model.ReasonPropertyNullableInConsumerNotNullableInProvider && onRequest:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" may send null, but "), breakreport.Name(provider), breakreport.Plain(" doesn't accept it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" doesn't accept null, but "), breakreport.Name(consumer), breakreport.Plain(" may send it")},
			consumerFix:     "stop sending null",
			providerFix:     "handle null in the code, then mark it nullable",
		}
	case reason == model.ReasonPropertyMemberMissingInProvider && onRequest:
		phrases = propertyBreakPhrases{
			consumerChecked: breakreport.Phrase{breakreport.Name(consumer), breakreport.Plain(" sends " + member + ", but "), breakreport.Name(provider), breakreport.Plain(" doesn't accept it")},
			providerChecked: breakreport.Phrase{breakreport.Name(provider), breakreport.Plain(" doesn't accept " + member + ", but "), breakreport.Name(consumer), breakreport.Plain(" sends it")},
			consumerFix:     "stop sending " + member,
			providerFix:     "accept " + member,
		}
	default:
		return fieldBlock{}, false
	}

	checkedField, otherField := property.checkedAndOtherFields()
	direction := "response"
	if onRequest {
		direction = restmodel.RequestInteraction
	}

	block := fieldBlock{
		fieldBlockKey: fieldBlockKey{
			field:        cmp.Or(checkedField, otherField, propertyPathPhrase(property.path)),
			reason:       reason,
			consumerType: consumerType,
			providerType: providerType,
			propertyType: details["propertyType"],
			member:       details["member"],
			direction:    direction,
			role:         contractBreak.Role,
		},
		places: []blockPlace{{
			Place: Place{
				Method:           checked.method,
				Endpoint:         checked.endpoint,
				Interaction:      interactionPhrase(checked.interaction),
				Property:         propertyPathPhrase(property.path),
				CounterpartField: otherField,
			},
			interaction: checked.interaction,
			property:    property.path,
		}},
	}

	consumerFix := breakreport.Fix{Participant: consumer, Action: phrases.consumerFix, Checked: asConsumer}
	providerFix := breakreport.Fix{Participant: provider, Action: phrases.providerFix, Checked: asProvider}
	if asConsumer {
		block.sentence, block.checkedFix, block.otherFix = phrases.consumerChecked, consumerFix, providerFix
	} else {
		block.sentence, block.checkedFix, block.otherFix = phrases.providerChecked, providerFix, consumerFix
	}

	return block, true
}

func mergeFieldBlocks(blocks []fieldBlock) []fieldBlock {
	slices.SortFunc(blocks, func(a, b fieldBlock) int {
		return cmp.Or(
			compareFieldBlockKeys(a.fieldBlockKey, b.fieldBlockKey),
			comparePlaces(a.places[0], b.places[0]),
			strings.Compare(a.places[0].CounterpartField, b.places[0].CounterpartField),
		)
	})
	// A place repeated in a block shows once, with the first of its counterpart fields.
	blocks = slices.CompactFunc(blocks, func(a, b fieldBlock) bool {
		return a.fieldBlockKey == b.fieldBlockKey && comparePlaces(a.places[0], b.places[0]) == 0
	})

	var merged []fieldBlock
	for _, block := range blocks {
		if len(merged) > 0 && merged[len(merged)-1].fieldBlockKey == block.fieldBlockKey {
			last := &merged[len(merged)-1]
			last.places = append(last.places, block.places...)
		} else {
			merged = append(merged, block)
		}
	}
	return merged
}

func compareFieldBlockKeys(a, b fieldBlockKey) int {
	return cmp.Or(
		strings.Compare(a.field, b.field),
		strings.Compare(string(a.reason), string(b.reason)),
		strings.Compare(a.consumerType, b.consumerType),
		strings.Compare(a.providerType, b.providerType),
		strings.Compare(a.propertyType, b.propertyType),
		strings.Compare(a.member, b.member),
		strings.Compare(a.direction, b.direction),
		strings.Compare(string(a.role), string(b.role)),
	)
}

func comparePlaces(a, b blockPlace) int {
	return cmp.Or(
		strings.Compare(a.Endpoint, b.Endpoint),
		strings.Compare(a.Method, b.Method),
		compareInteractions(a.interaction, b.interaction),
		strings.Compare(a.property, b.property),
	)
}
