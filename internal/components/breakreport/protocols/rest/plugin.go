package rest

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

const bodyPropertyPath = "$"

type Resources struct {
	Counterpart string
	Endpoints   []Endpoint
	Fields      []FieldBlock
}

type Endpoint struct {
	Method string
	Path   string
	Breaks []EndpointBreak
}

type EndpointBreak struct {
	Reason      model.BreakingReason
	Interaction string
	Sentence    breakreport.Phrase
	ReasonLine  string
	Fixes       []breakreport.Fix
}

type FieldBlock struct {
	Field    string
	Reason   model.BreakingReason
	Sentence breakreport.Phrase
	Fixes    []breakreport.Fix
	Places   []Place
}

type Place struct {
	Method           string
	Endpoint         string
	Interaction      string
	Property         string
	CounterpartField string
}

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (this *Plugin) Protocol() model.Protocol {
	return restmodel.Protocol
}

func (this *Plugin) ReportKey() string {
	return restmodel.ReportKey
}

func (this *Plugin) ToSection(checked breakreport.CheckedCounterpart, tree json.RawMessage) (breakreport.Section, error) {
	var endpoints restmodel.ReportTree
	if tree != nil {
		if err := json.Unmarshal(tree, &endpoints); err != nil {
			return breakreport.Section{}, fmt.Errorf("error decoding the endpoints tree: %w", err)
		}
	}

	resources := toResources(checked, endpoints)
	section := breakreport.Section{Display: resources, Breaks: len(resources.Fields)}
	for _, endpoint := range resources.Endpoints {
		section.Breaks += len(endpoint.Breaks)
	}
	for _, field := range resources.Fields {
		section.Places += len(field.Places)
	}

	return section, nil
}

func toResources(checked breakreport.CheckedCounterpart, endpoints restmodel.ReportTree) Resources {
	resources := Resources{Counterpart: checked.Counterpart.Name}

	var blocks []fieldBlock
	for _, path := range slices.Sorted(maps.Keys(endpoints)) {
		methods := endpoints[path]
		for _, method := range slices.Sorted(maps.Keys(methods)) {
			interaction := checkedInteraction{
				CheckedCounterpart: checked,
				method:             strings.ToUpper(method),
				endpoint:           path,
			}

			removed, interactions := splitRemovedBreak(methods[method])
			methodBlocks, interactions := splitFieldBlocks(interaction, interactions)
			blocks = append(blocks, methodBlocks...)
			if removed == nil && len(interactions) == 0 {
				continue
			}

			endpoint := Endpoint{Method: interaction.method, Path: path}
			if removed != nil {
				endpoint.Breaks = append(endpoint.Breaks, toEndpointBreak(interaction, *removed))
			}

			for _, name := range sortedInteractions(interactions) {
				interaction.interaction = name
				for _, contractBreak := range interactions[name] {
					broken := toEndpointBreak(interaction, contractBreak)
					broken.Interaction = interactionPhrase(name)
					endpoint.Breaks = append(endpoint.Breaks, broken)
				}
			}

			resources.Endpoints = append(resources.Endpoints, endpoint)
		}
	}

	for _, block := range mergeFieldBlocks(blocks) {
		resources.Fields = append(resources.Fields, block.toFieldBlock())
	}

	return resources
}

type checkedInteraction struct {
	breakreport.CheckedCounterpart
	method      string
	endpoint    string
	interaction string
}

func (this checkedInteraction) isRequest() bool {
	return this.interaction == restmodel.RequestInteraction
}

func (this checkedInteraction) resource() string {
	return this.method + " " + this.endpoint
}

// The broker sends the removed-endpoint break once per consumed interaction, but it is about
// the whole endpoint. Interactions left without breaks are dropped.
func splitRemovedBreak(interactions map[string][]model.ReportBreak) (*model.ReportBreak, map[string][]model.ReportBreak) {
	var removed *model.ReportBreak
	remaining := make(map[string][]model.ReportBreak, len(interactions))
	for _, interaction := range sortedInteractions(interactions) {
		var kept []model.ReportBreak
		for _, contractBreak := range interactions[interaction] {
			if contractBreak.Reason != model.ReasonProviderResourceRemovedButStillConsumed {
				kept = append(kept, contractBreak)
			} else if removed == nil {
				removed = &contractBreak
			}
		}
		if len(kept) > 0 {
			remaining[interaction] = kept
		}
	}
	return removed, remaining
}

func splitFieldBlocks(checked checkedInteraction, interactions map[string][]model.ReportBreak) ([]fieldBlock, map[string][]model.ReportBreak) {
	var blocks []fieldBlock
	remaining := make(map[string][]model.ReportBreak, len(interactions))
	for _, interaction := range sortedInteractions(interactions) {
		checked.interaction = interaction
		for _, contractBreak := range interactions[interaction] {
			if block, named := toFieldBlock(checked, contractBreak); named {
				blocks = append(blocks, block)
			} else {
				remaining[interaction] = append(remaining[interaction], contractBreak)
			}
		}
	}
	return blocks, remaining
}

func sortedInteractions(interactions map[string][]model.ReportBreak) []string {
	return slices.SortedFunc(maps.Keys(interactions), compareInteractions)
}

func compareInteractions(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == restmodel.RequestInteraction:
		return -1
	case b == restmodel.RequestInteraction:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func interactionPhrase(interaction string) string {
	if interaction == restmodel.RequestInteraction {
		return restmodel.RequestInteraction
	}
	return "response " + interaction
}

func propertyPathPhrase(path string) string {
	if path == bodyPropertyPath {
		return "the body"
	}
	return fmt.Sprintf("%q", path)
}
