package breakreport

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	requestInteraction = "request"
	bodyPropertyPath   = "$"
)

type StoredCheck struct {
	Participant  string
	Environment  string
	Report       []byte
	Counterparts []StoredCounterpart
}

type StoredCounterpart struct {
	Name       string
	Role       model.CounterpartRole
	Version    string
	Registered bool
	Deployable bool
}

type Report struct {
	Stored               bool
	Breaks               int
	CounterpartsBreaking int
	Places               int
	Counterparts         []Counterpart
}

type Counterpart struct {
	Name       string
	Role       model.CounterpartRole
	Version    string
	Registered bool
	Deployable bool
	Breaks     int
	Endpoints  []Endpoint
	Fields     []FieldBlock
}

type Endpoint struct {
	Method string
	Path   string
	Breaks []EndpointBreak
}

type EndpointBreak struct {
	Reason      model.BreakingReason
	Interaction string
	Sentence    Phrase
	ReasonLine  string
	Fixes       []Fix
}

type FieldBlock struct {
	Field    string
	Reason   model.BreakingReason
	Sentence Phrase
	Fixes    []Fix
	Places   []Place
}

type Place struct {
	Method           string
	Endpoint         string
	Interaction      string
	Property         string
	CounterpartField string
}

type Fix struct {
	Participant string
	Action      string
	Checked     bool
}

type storedResult struct {
	Endpoints map[string]map[string]map[string][]storedBreak `json:"endpoints"`
}

type storedBreak struct {
	Reason  model.BreakingReason `json:"reason"`
	Role    model.BreakRole      `json:"role"`
	Details map[string]string    `json:"details"`
}

func FromStoredCheck(check StoredCheck) (Report, error) {
	counterparts := slices.SortedFunc(slices.Values(check.Counterparts), func(a, b StoredCounterpart) int {
		return strings.Compare(a.Name, b.Name)
	})

	if check.Report == nil {
		report := Report{Counterparts: make([]Counterpart, 0, len(counterparts))}
		for _, counterpart := range counterparts {
			report.Counterparts = append(report.Counterparts, Counterpart{
				Name:       counterpart.Name,
				Version:    counterpart.Version,
				Registered: counterpart.Registered,
				Deployable: counterpart.Deployable,
			})
		}
		return report, nil
	}

	var results map[string]storedResult
	if err := json.Unmarshal(check.Report, &results); err != nil {
		return Report{}, fmt.Errorf("error decoding stored check report: %w", err)
	}

	report := Report{Stored: true, Counterparts: make([]Counterpart, 0, len(counterparts))}
	for _, stored := range counterparts {
		counterpart := toCounterpart(check, stored, results[stored.Name])

		report.Breaks += counterpart.Breaks
		if counterpart.Breaks > 0 {
			report.CounterpartsBreaking++
		}
		for _, field := range counterpart.Fields {
			report.Places += len(field.Places)
		}

		report.Counterparts = append(report.Counterparts, counterpart)
	}

	return report, nil
}

func toCounterpart(check StoredCheck, stored StoredCounterpart, result storedResult) Counterpart {
	counterpart := Counterpart{
		Name:       stored.Name,
		Role:       stored.Role,
		Version:    stored.Version,
		Registered: stored.Registered,
		Deployable: stored.Deployable,
	}

	var blocks []fieldBlock
	for _, path := range slices.Sorted(maps.Keys(result.Endpoints)) {
		methods := result.Endpoints[path]
		for _, method := range slices.Sorted(maps.Keys(methods)) {
			checked := checkedInteraction{
				participant: check.Participant,
				counterpart: stored,
				environment: check.Environment,
				method:      strings.ToUpper(method),
				endpoint:    path,
			}

			removed, interactions := splitRemovedBreak(methods[method])
			methodBlocks, interactions := splitFieldBlocks(checked, interactions)
			blocks = append(blocks, methodBlocks...)
			if removed == nil && len(interactions) == 0 {
				continue
			}

			endpoint := Endpoint{Method: checked.method, Path: path}
			if removed != nil {
				endpoint.Breaks = append(endpoint.Breaks, toEndpointBreak(checked, *removed))
			}

			for _, interaction := range sortedInteractions(interactions) {
				checked.interaction = interaction
				for _, contractBreak := range interactions[interaction] {
					broken := toEndpointBreak(checked, contractBreak)
					broken.Interaction = interactionPhrase(interaction)
					endpoint.Breaks = append(endpoint.Breaks, broken)
				}
			}

			counterpart.Breaks += len(endpoint.Breaks)
			counterpart.Endpoints = append(counterpart.Endpoints, endpoint)
		}
	}

	for _, block := range mergeFieldBlocks(blocks) {
		counterpart.Fields = append(counterpart.Fields, block.toFieldBlock())
	}
	counterpart.Breaks += len(counterpart.Fields)

	return counterpart
}

type checkedInteraction struct {
	participant string
	counterpart StoredCounterpart
	environment string
	method      string
	endpoint    string
	interaction string
}

func (this checkedInteraction) isRequest() bool {
	return this.interaction == requestInteraction
}

func (this checkedInteraction) resource() string {
	return this.method + " " + this.endpoint
}

// The broker sends the removed-endpoint break once per consumed interaction, but it is about
// the whole endpoint. Interactions left without breaks are dropped.
func splitRemovedBreak(interactions map[string][]storedBreak) (*storedBreak, map[string][]storedBreak) {
	var removed *storedBreak
	remaining := make(map[string][]storedBreak, len(interactions))
	for _, interaction := range sortedInteractions(interactions) {
		var kept []storedBreak
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

func splitFieldBlocks(checked checkedInteraction, interactions map[string][]storedBreak) ([]fieldBlock, map[string][]storedBreak) {
	var blocks []fieldBlock
	remaining := make(map[string][]storedBreak, len(interactions))
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

func sortedInteractions(interactions map[string][]storedBreak) []string {
	return slices.SortedFunc(maps.Keys(interactions), compareInteractions)
}

func compareInteractions(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == requestInteraction:
		return -1
	case b == requestInteraction:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func interactionPhrase(interaction string) string {
	if interaction == requestInteraction {
		return requestInteraction
	}
	return "response " + interaction
}

func propertyPathPhrase(path string) string {
	if path == bodyPropertyPath {
		return "the body"
	}
	return fmt.Sprintf("%q", path)
}
